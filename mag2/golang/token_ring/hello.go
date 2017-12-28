package main

import (
    "math/rand"
    "fmt"
    "strconv"
    "net"
    "time"
    "os"
    "encoding/json"
)

const FIRST_PORT int = 30000
const MASTER_FIRST_PORT int = 40000
const read_deadline_time time.Duration = 1e4

type Node struct {
    id   int
    port int
    master_port int
}

type msg_t struct {
    Origin int
    // if Useful == true then is't message with data, else it's notification message, Data = ""
    Useful bool
    Data string
}

type manage_msg_t struct {
    Type string
    Dst int
    Data string
}

type token_t struct {
    // if Dst == -1 then empty token, Data in Msg = ""
    Dst int
    // src is the last vertex, but not the origin!
    Src int
    Msg msg_t
}

type Graph map[Node][]Node

func newNode(a int) Node {
    return Node{
        id:   a,
        port: a + FIRST_PORT,
        master_port: a + MASTER_FIRST_PORT,
    }
}

func (g Graph) addEdge(a, b Node) {
    g[a] = append(g[a], b)
    //g[b] = append(g[b], a)
}

func (g Graph) Neighbors(id int) ([]Node, bool) {
    node := newNode(id)
    nodes, ok := g[node]
    return nodes, ok
}

func generate_ring(n int, port int) Graph {
    g := make(Graph)
    perm := rand.Perm(n)
    for i := 0; i < n; i++ {
        g.addEdge(newNode(perm[i]), newNode(perm[(i + 1) % n]))
    }
    
    return g
}

func ErrHandler(err error) {
    if (err != nil) {
        //fmt.Println("Error = ", err)
        //os.Exit(0)
    }
}

func send(token token_t, port int, last_send_time *int64, last_send_token *token_t) {
    dstAddr, _ := net.ResolveUDPAddr("udp","127.0.0.1:"+strconv.Itoa(port))
    srcAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
    Conn, err := net.DialUDP("udp", srcAddr, dstAddr)
    ErrHandler(err)
    if (Conn == nil) {
        fmt.Println("Conn = nil")
    }
    byte_msg, err := json.Marshal(token)
    if (err != nil) {
        fmt.Printf("Marshal error = ", err)
    }
    Conn.Write(byte_msg)
    Conn.Close()
    *last_send_time = time.Now().UnixNano()
    *last_send_token = token
}

func node_token_ring_emulator(v Node, g Graph, t int, n int) {
    drop_flag := false
    // listen port
    Addr, err := net.ResolveUDPAddr("udp","127.0.0.1:"+strconv.Itoa(v.port))
    ErrHandler(err)
    Conn, err := net.ListenUDP("udp", Addr)
    ErrHandler(err)
    defer Conn.Close()
    // listen master port
    Addr2, err2 := net.ResolveUDPAddr("udp","127.0.0.1:"+strconv.Itoa(v.master_port))
    ErrHandler(err2)
    Conn2, err2 := net.ListenUDP("udp", Addr2)
    ErrHandler(err2)
    defer Conn2.Close()
    
    q := make([]token_t, 0)
    last_send_token := token_t{}
    last_send_time := time.Now().UnixNano()
    if (v.id == 0) {
        // let's 0 node send some message
        dst := 1
        src := 0
        q = append(q, token_t{dst, src, msg_t{0, true, "123"}})
        start_token := q[0]
        send(start_token, g[v][0].port, &last_send_time, &last_send_token)
    }
    byte_buf := make([]byte, 1000)
    for {
        Conn.SetReadDeadline(time.Now().Add(time.Nanosecond * read_deadline_time))
        cnt, _, err := Conn.ReadFromUDP(byte_buf)
        ErrHandler(err)
        token := token_t{}
        json.Unmarshal(byte_buf[:cnt], &token)
        if (cnt != 0) {
            if (token.Dst == -1) {
                fmt.Println("node ", v.id, " recieved token from node ", token.Src, " sending token to node ", g[v][0].id)
            } else if (token.Msg.Useful) {
                fmt.Println("node ", v.id, " recieved token from node ", token.Src, " with data from node ", token.Msg.Origin, " (data=''", token.Msg.Data, "''), ", "sending token to node ", g[v][0].id)
            } else {
                fmt.Println("node ", v.id, " recieved token from node ", token.Src, " with delivery confirmation from node ", token.Msg.Origin, ", sending token to node ", g[v][0].id)
            }
            if (!drop_flag) {
                if (token.Dst == v.id) {
                    if (token.Msg.Useful) {
                        // useful message, read if we want
                        // and then clear data
                        token.Dst = token.Msg.Origin
                        token.Msg.Data = ""
                        token.Msg.Useful = false
                        token.Msg.Origin = v.id
                    } else {
                        // free token
                        token.Dst = -1
                        for i := range q {
                            if (q[i] == last_send_token) {
                                q = append(q[:i], q[i+1:]...)
                                break
                            }
                        }
                    }
                }
                if (token.Dst == -1 && len(q) > 0) {
                    token.Dst = q[len(q) - 1].Dst
                    token.Msg.Origin = q[len(q) - 1].Msg.Origin
                    token.Msg.Useful = true
                    token.Msg.Data = q[len(q) - 1].Msg.Data
                }
                time.Sleep(time.Nanosecond * 1e6 * time.Duration(t))
                token.Src = v.id
                send(token, g[v][0].port, &last_send_time, &last_send_token)
            } else {
                fmt.Println("drop")
                drop_flag = false
            }
        }
        if (v.id == 0 && (time.Now().UnixNano() - last_send_time) > 1e6 * int64(2 * n) * int64(t)) {
            fmt.Println("restart sending from 0 node")
            send(last_send_token, g[v][0].port, &last_send_time, &last_send_token)
        }
        // listen master port
        Conn2.SetReadDeadline(time.Now().Add(time.Nanosecond * read_deadline_time))
        cnt2, _, err2 := Conn2.ReadFromUDP(byte_buf)
        ErrHandler(err2)
        manage_msg := manage_msg_t{}
        json.Unmarshal(byte_buf[:cnt2], &manage_msg)
        if (cnt2 != 0) {
            fmt.Println("node ", v.id, ": recieved service message: {\"type\":", manage_msg.Type, ",\"dst\":", manage_msg.Dst, ",\"data\":\"", manage_msg.Data, "\"}")
            if (manage_msg.Type == "send") {
                q = append(q, token_t{manage_msg.Dst, v.id, msg_t{v.id, true, manage_msg.Data}})
            } else if (manage_msg.Type == "drop") {
                drop_flag = true
            }
        }
    }
}

func main() {
    n := 10
    t := 1000
    if (len(os.Args) >= 3) {
        if (os.Args[1] == "--n") {
            n, _ = strconv.Atoi(os.Args[2])
        } else if (os.Args[1] == "--t") {
            t, _ = strconv.Atoi(os.Args[2])
        }
    }
    if (len(os.Args) >= 5) {
        if (os.Args[3] == "--n") {
            n, _ = strconv.Atoi(os.Args[4])
        } else if (os.Args[3] == "--t") {
            t, _ = strconv.Atoi(os.Args[4])
        }
    }
    fmt.Printf("n = %d\n", n)
    fmt.Printf("t = %d\n", t)
    var g = generate_ring(n, FIRST_PORT)
    for v := range g {
        go node_token_ring_emulator(v, g, t, n)
    }
    time.Sleep(time.Second * 100000)
    fmt.Println("done")
}



