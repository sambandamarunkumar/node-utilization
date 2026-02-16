import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	initialNodes = 3
	maxNodes     = 11
	records      = 10000
	workers      = 16
	iterations   = 50000
	interval     = 2
)

type Node struct {
	data map[int]int
	load int64
	mu   sync.RWMutex
}

var cluster []*Node
var routing map[int]int
var committed int64
var nodeCount int64

func newNode() *Node {
	return &Node{data: make(map[int]int)}
}

func route(key int) int {
	return routing[key]
}

func execute(key int) {
	id := route(key)
	n := cluster[id]

	n.mu.Lock()
	n.data[key]++
	n.mu.Unlock()

	atomic.AddInt64(&n.load, 1)
	atomic.AddInt64(&committed, 1)
}

func worker(wg *sync.WaitGroup) {
	for i := 0; i < iterations; i++ {
		k := rand.Intn(records)
		execute(k)
	}
	wg.Done()
}

func utilization() []float64 {
	stats := make([]float64, len(cluster))
	var total int64
	for _, n := range cluster {
		total += atomic.LoadInt64(&n.load)
	}
	if total == 0 {
		return stats
	}
	for i, n := range cluster {
		stats[i] = float64(atomic.LoadInt64(&n.load)) / float64(total)
	}
	return stats
}

func rebalance() {
	stats := utilization()

	min := 0
	max := 0

	for i := 1; i < len(stats); i++ {
		if stats[i] < stats[min] {
			min = i
		}
		if stats[i] > stats[max] {
			max = i
		}
	}

	for k := 0; k < records/20; k++ {
		key := rand.Intn(records)
		routing[key] = min
	}
}

func scale() {
	stats := utilization()
	var maxUtil float64
	for _, u := range stats {
		if u > maxUtil {
			maxUtil = u
		}
	}

	if maxUtil > 0.35 && len(cluster) < maxNodes {
		cluster = append(cluster, newNode())
		atomic.AddInt64(&nodeCount, 1)
	}

	if maxUtil < 0.15 && len(cluster) > initialNodes {
		cluster = cluster[:len(cluster)-1]
		atomic.AddInt64(&nodeCount, -1)
	}
}

func controller(stop chan bool) {
	ticker := time.NewTicker(time.Second * interval)
	for {
		select {
		case <-ticker.C:
			rebalance()
			scale()
		case <-stop:
			return
		}
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	cluster = []*Node{}
	routing = make(map[int]int)

	for i := 0; i < initialNodes; i++ {
		cluster = append(cluster, newNode())
	}

	atomic.StoreInt64(&nodeCount, int64(initialNodes))

	for k := 0; k < records; k++ {
		routing[k] = k % initialNodes
	}

	stop := make(chan bool)
	go controller(stop)

	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker(&wg)
	}

	wg.Wait()
	stop <- true

	elapsed := time.Since(start).Seconds()
	tps := float64(committed) / elapsed

	fmt.Println("Transactions:", committed)
	fmt.Println("Time(sec):", elapsed)
	fmt.Println("Throughput:", int(tps))
	fmt.Println("Nodes:", nodeCount)

	for i, n := range cluster {
		fmt.Println("Node", i, "Load:", n.load)
	}
}
