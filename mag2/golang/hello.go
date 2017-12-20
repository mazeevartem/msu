package main

import (
	"math/rand"
	"strconv"
    "fmt"
    "net"
    "time"
    "os"
    "encoding/json"
)

const N int = 32
const MIN_DEGREE int = 5
const MAX_DEGREE int = 10
const FIRST_PORT int = 50000
const delta_t_nanoseconds int64 = 1e6
const read_deadline_time time.Duration = 1e4

type Node struct {
	id   int
	port int
}

type msg_t struct {
    Id int
    T string
    Sender int
    Origin int
    Data string
}

type Graph map[Node][]Node

var basePort int

func newNode(a int) Node {
	return Node{
		id:   a,
		port: a + basePort,
	}
}

func (n Node) String() string {
	return strconv.Itoa(n.id)
}

func (n Node) Port() int {
	return n.port
}

func (g Graph) addEdge(a, b Node) {
	g[a] = append(g[a], b)
	g[b] = append(g[b], a)
}

func (g Graph) hasEdge(a, b Node) bool {
	if nodes, ok := g[a]; ok {
		for _, node := range nodes {
			if node == b {
				return true
			}
		}
	}
	return false
}

func (g Graph) Neighbors(id int) ([]Node, bool) {
	node := newNode(id)
	nodes, ok := g[node]
	return nodes, ok
}

func (g Graph) GetNode(id int) (Node, bool) {
	node := newNode(id)
	_, ok := g[node]
	return node, ok
}

// Generate generates connected random graph with n nodes.
// The degree of most part of vertices will be in interval [minDegree:maxDegree]
// some (not many) vertices can have more than maxDegree vertices.
//
// Port value is used as a base value for a port
// each generated node i (0 <= 0 <= n) will be associated
// with port p (p = i + port)
//
// Generate uses standard golang rand package,
// it is user's responsibility to init rand with a propper seed.
func Generate(n, minDegree, maxDegree, port int) Graph {
	if minDegree <= 0 || minDegree > maxDegree {
		panic("wrong minDegree or maxDegree value")
	}

	basePort = port
	g := make(Graph)

	degrees := make(map[int]int, n)
	for i := 0; i < n; i++ {
		degrees[i] = rand.Intn(maxDegree-minDegree) + minDegree
	}

	var nodes []int
	for len(degrees) > 1 {
		nodes = nodes[:0]
		for node := range degrees {
			nodes = append(nodes, node)
		}

		cur, nodes := nodes[0], nodes[1:]
		perm := rand.Perm(len(nodes))

		max := degrees[cur]
		if max > len(perm) {
			max = len(perm)
		}
		a := newNode(cur)
		for index := range perm[:max] {
			neigh := nodes[index]
			degrees[neigh]--
			if degrees[neigh] <= 0 {
				delete(degrees, neigh)
			}
			b := newNode(neigh)
			g.addEdge(a, b)
		}
		delete(degrees, cur)
	}

	for v := range degrees {
		a := newNode(v)
		if len(g[a]) > 0 {
			continue
		}
		b := v
		for b == v {
			b = rand.Intn(n)
		}
		g.addEdge(a, newNode(b))
	}

	perm := rand.Perm(n)
	for i := 1; i < n; i++ {
		a := perm[i-1]
		b := perm[i]
		na, nb := newNode(a), newNode(b)
		if !g.hasEdge(na, nb) {
			g.addEdge(na, nb)
		}
		degrees[a]--
		degrees[b]--
		if degrees[a] <= 0 {
			delete(degrees, a)
		}
		if degrees[b] <= 0 {
			delete(degrees, b)
		}
	}
	return g
}

type Pair struct {
    a msg_t
    b int64
}

func ErrHandler(err error) {
    if (err != nil) {
        fmt.Println("Error = ", err)
        //os.Exit(0)
    }
}

func node_gossip_emulator(v Node, g Graph) {
    var cnt int = 0
    ok_node := make([]bool, N)
    for i := 0; i < N; i++ {
        ok_node[i] = false
    }
    var local_msg_id int = 1
    q := make([]Pair, 0)
    byte_buf := make([]byte, len(g[v]) * 1000)
    start_time := time.Now().UnixNano()
    if (v.id == 0) {
        q = append(q, Pair{msg_t{local_msg_id, "multicast", v.id, v.id, ""}, -1e18})
        local_msg_id++
    }
    Addr, err := net.ResolveUDPAddr("udp","127.0.0.1:"+strconv.Itoa(v.port))
    ErrHandler(err)
    Conn, err := net.ListenUDP("udp", Addr)
    ErrHandler(err)
    defer Conn.Close()
    for {
        Conn.SetReadDeadline(time.Now().Add(time.Nanosecond * read_deadline_time))
        n, _, err := Conn.ReadFromUDP(byte_buf)
        ErrHandler(err)
        msg := msg_t{}
        json.Unmarshal(byte_buf[:n], &msg)
        if (n != 0) {
            var new_msg bool = true
            for _, q_msg := range q {
                if (msg == q_msg.a) {
                    new_msg = false
                    break
                }
            }
            if (new_msg == true) {
                q = append(q, Pair{msg, -1e18})
                if (msg.T == "multicast") {
                    var notification_msg msg_t = msg_t{local_msg_id, "notification", v.id, v.id, ""}
                    local_msg_id++
                    q = append(q, Pair{notification_msg, -1e18})
                }
                if (v.id == 0 && msg.T == "notification") {
                    if (ok_node[msg.Origin] == false) {
                        ok_node[msg.Origin] = true
                        cnt++
                        fmt.Printf("cnt = %d\n", cnt)
                        if (cnt == N) {
                            fmt.Printf("answer T = ", (time.Now().UnixNano() - start_time) / delta_t_nanoseconds)
                            os.Exit(0)
                        }
                    }
                }
            }
        }
        
        for _, msg := range q {
            if (time.Now().UnixNano() - msg.b > delta_t_nanoseconds) {
                var idx = rand.Intn(len(g[v]))
                dstAddr, _ := net.ResolveUDPAddr("udp","127.0.0.1:"+strconv.Itoa(g[v][idx].port))
                srcAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
                Conn, err := net.DialUDP("udp", srcAddr, dstAddr)
                ErrHandler(err)
                if (Conn == nil) {
                    fmt.Println("Conn = nil")
                }
                cur_msg := msg.a
                cur_msg.Id = local_msg_id
                local_msg_id++
                cur_msg.Sender = v.id
                byte_msg, err := json.Marshal(cur_msg)
                if (err != nil) {
                    fmt.Printf("Marshal error = ", err)
                }
                Conn.Write(byte_msg)
                Conn.Close()
                msg.b = time.Now().UnixNano()
            }
        }
    }
}

func main() {
    fmt.Printf("hello, world\n")
    var g = Generate(N, MIN_DEGREE, MAX_DEGREE, FIRST_PORT)
    for v := range g {
        go node_gossip_emulator(v, g)
    }
    time.Sleep(time.Second * 100000)
    fmt.Println("done")
}



