import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	nodes      = 5
	records    = 10000
	workers    = 16
	iterations = 50000
)

type Partition struct {
	data map[int]int
	load int64
	mu   sync.RWMutex
}

var cluster []*Partition
var committed int64

func newPartition() *Partition {
	return &Partition{data: make(map[int]int)}
}

func route(key int) int {
	return key % nodes
}

func execute(key int) {
	id := route(key)
	p := cluster[id]

	p.mu.Lock()
	p.data[key]++
	p.mu.Unlock()

	atomic.AddInt64(&p.load, 1)
	atomic.AddInt64(&committed, 1)
}

func worker(wg *sync.WaitGroup) {
	for i := 0; i < iterations; i++ {
		k := rand.Intn(records)
		execute(k)
	}
	wg.Done()
}

func main() {
	rand.Seed(time.Now().UnixNano())

	cluster = make([]*Partition, nodes)
	for i := 0; i < nodes; i++ {
		cluster[i] = newPartition()
	}

	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker(&wg)
	}

	wg.Wait()

	elapsed := time.Since(start).Seconds()
	tps := float64(committed) / elapsed

	fmt.Println("Transactions:", committed)
	fmt.Println("Time(sec):", elapsed)
	fmt.Println("Throughput:", int(tps))

	for i := 0; i < nodes; i++ {
		fmt.Println("Node", i, "Load:", cluster[i].load)
	}
}
