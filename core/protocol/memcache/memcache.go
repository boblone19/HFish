package memcache

/**
 * Supported commands:
 * get <key>\r\n
 * set <key> <flags> <ttl> <size>\r\ndata\r\n
 * add <key> <flags> <ttl> <size>\r\ndata\r\n
 * delete <key>
 * flush_all
 * stats
 * stats slabs
 * stats malloc
 * stats items
 * stats detail
 * stats sizes
 * version
 * verbosity
 * quit
 */

import (
	"HFish/core/protocol/memcache/LinkedHashMap"
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var linkedHashMap = LinkedHashMap.NewLinkedHashMap()
var networkRx = 100
var networkTx = 150

var commands = map[string]func([]string) ([]byte, int){

	"quit": func(args []string) ([]byte, int) {
		return nil, -1
	},

	"verbosity": func(args []string) ([]byte, int) {
		if len(args) < 1 {
			return RESPONSE_ERROR, 0
		}
		return RESPONSE_OK, 0
	},

	"flush_all": func(args []string) ([]byte, int) {
		linkedHashMap.Lock()
		defer linkedHashMap.Unlock()

		linklist := linkedHashMap.GetLinkList()
		for {
			node := linklist.GetHead()
			if node == nil {
				break
			}
			key := node.GetVal().(string)
			nextNode := node.GetNext()
			linkedHashMap.Remove(key)
			node = nextNode
		}

		return RESPONSE_OK, 0
	},

	"version": func(args []string) ([]byte, int) {
		if len(args) != 0 {
			return RESPONSE_ERROR, 0
		}
		return RESPONSE_VERSION, 0
	},

	"get": func(args []string) ([]byte, int) {
		if len(args) < 1 {
			return RESPONSE_ERROR, 0
		}

		linkedHashMap.RLock()
		defer linkedHashMap.RUnlock()

		val := linkedHashMap.Get(args[0])
		if val == nil {
			return RESPONSE_END, 0
		}
		return append([]byte(val.(string)+"\r\n"), RESPONSE_END...), 0
	},

	"set": func(args []string) ([]byte, int) {
		if len(args) == 5 {
			key := args[0]
			data := args[4]
			linkedHashMap.Lock()
			defer linkedHashMap.Unlock()
			items := linkedHashMap.Len()
			for ; items > 60; items-- {
				_, _ = linkedHashMap.Remove(linkedHashMap.GetLinkList().GetHead().GetVal().(string))
			}
			linkedHashMap.Add(key, data)
			return []byte("STORED\r\n"), 0
		}

		if len(args) == 4 {
			flags := args[1]
			ttl := args[2]
			size := args[3]

			_, err := strconv.Atoi(flags)
			if err != nil {
				return append(RESPONSE_CLIENT_ERROR, RESPONSE_BAD_COMMAND_LINE...), 0
			}
			_, err = strconv.Atoi(ttl)
			if err != nil {
				return append(RESPONSE_CLIENT_ERROR, RESPONSE_BAD_COMMAND_LINE...), 0
			}
			parsedSize, err := strconv.Atoi(size)
			if err != nil {
				return append(RESPONSE_CLIENT_ERROR, RESPONSE_BAD_COMMAND_LINE...), 0
			}
			if parsedSize > 1500 {
				return append(RESPONSE_SERVER_ERROR, RESPONSE_OBJECT_TOO_LARGE...), 0
			}
			return nil, parsedSize
		}

		return RESPONSE_ERROR, 0
	},

	"add": func(args []string) ([]byte, int) {
		if len(args) == 5 {
			key := args[0]
			data := args[4]
			linkedHashMap.Lock()
			defer linkedHashMap.Unlock()
			if linkedHashMap.Get(key) != nil {
				return []byte("NOT_STORED\r\n"), 0
			}
			items := linkedHashMap.Len()
			for ; items > 60; items-- {
				fmt.Println(items)
				_, _ = linkedHashMap.Remove(linkedHashMap.GetLinkList().GetHead().GetVal().(string))
			}
			linkedHashMap.Add(key, data)
			return []byte("STORED\r\n"), 0
		}

		if len(args) == 4 {
			flags := args[1]
			ttl := args[2]
			size := args[3]

			_, err := strconv.Atoi(flags)
			if err != nil {
				return append(RESPONSE_CLIENT_ERROR, RESPONSE_BAD_COMMAND_LINE...), 0
			}
			_, err = strconv.Atoi(ttl)
			if err != nil {
				return append(RESPONSE_CLIENT_ERROR, RESPONSE_BAD_COMMAND_LINE...), 0
			}
			parsedSize, err := strconv.Atoi(size)
			if err != nil {
				return append(RESPONSE_CLIENT_ERROR, RESPONSE_BAD_COMMAND_LINE...), 0
			}
			if parsedSize > 1024 {
				return append(RESPONSE_SERVER_ERROR, RESPONSE_OBJECT_TOO_LARGE...), 0
			}
			return nil, parsedSize
		}

		return RESPONSE_ERROR, 0
	},

	"delete": func(args []string) ([]byte, int) {
		if len(args) < 1 {
			return RESPONSE_ERROR, 0
		}

		linkedHashMap.Lock()
		defer linkedHashMap.Unlock()

		result, _ := linkedHashMap.Remove(args[0])
		if !result {
			return RESPONSE_NOT_FOUND, 0
		}
		return RESPONSE_DELETED, 0
	},

	"stats": func(args []string) ([]byte, int) {
		nowTime := time.Now()
		networkRx = networkRx + randNumber(10, 50)
		networkTx = networkTx + randNumber(100, 500)
		linkedHashMap.RLock()
		defer linkedHashMap.RUnlock()
		items := linkedHashMap.Len()

		if len(args) == 0 {
			statsArray := []string{
				"STAT pid 1\r\n",
				fmt.Sprintf("STAT uptime %d\r\n", int(nowTime.Sub(UPTIME)/time.Second)),
				fmt.Sprintf("STAT time %d\r\n", nowTime.Unix()),
				"STAT version 1.5.16\r\n",
				"STAT libevent 2.1.8-stable\r\n",
				"STAT pointer_size 64\r\n",
				"STAT rusage_user 0.029000\r\n",
				"STAT rusage_system 0.029000\r\n",
				"STAT max_connections 1024\r\n",
				fmt.Sprintf("STAT curr_connections %d\r\n", randNumber(1, 5)),
				"STAT rejected_connections 0\r\n",
				"STAT connection_structures 3\r\n",
				"STAT reserved_fds 20\r\n",
				"STAT cmd_get 0\r\n",
				"STAT cmd_set 0\r\n",
				"STAT cmd_flush 0\r\n",
				"STAT cmd_touch 0\r\n",
				"STAT get_hits 0\r\n",
				"STAT get_misses 0\r\n",
				"STAT get_expired 0\r\n",
				"STAT get_flushed 0\r\n",
				"STAT delete_misses 0\r\n",
				"STAT delete_hits 0\r\n",
				"STAT incr_misses 0\r\n",
				"STAT incr_hits 0\r\n",
				"STAT decr_misses 0\r\n",
				"STAT decr_hits 0\r\n",
				"STAT cas_misses 0\r\n",
				"STAT cas_hits 0\r\n",
				"STAT cas_badval 0\r\n",
				"STAT touch_hits 0\r\n",
				"STAT touch_misses 0\r\n",
				"STAT auth_cmds 0\r\n",
				"STAT auth_errors 0\r\n",
				fmt.Sprintf("STAT bytes_read %d\r\n", networkRx),
				fmt.Sprintf("STAT bytes_written %d\r\n", networkTx),
				"STAT limit_maxbytes 67108864\r\n",
				"STAT accepting_conns 1\r\n",
				"STAT listen_disabled_num 0\r\n",
				"STAT time_in_listen_disabled_us 0\r\n",
				"STAT threads 4\r\n",
				"STAT conn_yields 0\r\n",
				"STAT hash_power_level 16\r\n",
				"STAT hash_bytes 524288\r\n",
				"STAT hash_is_expanding 0\r\n",
				"STAT slab_reassign_rescues 0\r\n",
				"STAT slab_reassign_chunk_rescues 0\r\n",
				"STAT slab_reassign_evictions_nomem 0\r\n",
				"STAT slab_reassign_inline_reclaim 0\r\n",
				"STAT slab_reassign_busy_items 0\r\n",
				"STAT slab_reassign_busy_deletes 0\r\n",
				"STAT slab_reassign_running 0\r\n",
				"STAT slabs_moved 0\r\n",
				"STAT lru_crawler_running 0\r\n",
				"STAT lru_crawler_starts 510\r\n",
				"STAT lru_maintainer_juggles 224\r\n",
				"STAT malloc_fails 0\r\n",
				"STAT log_worker_dropped 0\r\n",
				"STAT log_worker_written 0\r\n",
				"STAT log_watcher_skipped 0\r\n",
				"STAT log_watcher_sent 0\r\n",
				"STAT bytes 0\r\n",
				fmt.Sprintf("STAT curr_items %d\r\n", items),
				"STAT total_items 0\r\n",
				"STAT slab_global_page_pool 0\r\n",
				"STAT expired_unfetched 0\r\n",
				"STAT evicted_unfetched 0\r\n",
				"STAT evicted_active 0\r\n",
				"STAT evictions 0\r\n",
				"STAT reclaimed 0\r\n",
				"STAT crawler_reclaimed 0\r\n",
				"STAT crawler_items_checked 0\r\n",
				"STAT lrutail_reflocked 0\r\n",
				"STAT moves_to_cold 0\r\n",
				"STAT moves_to_warm 0\r\n",
				"STAT moves_within_lru 0\r\n",
				"STAT direct_reclaims 0\r\n",
				"STAT lru_bumps_dropped 0\r\n",
				"END\r\n",
			}
			responseBuffer := []byte{}
			for _, response := range statsArray {
				responseBuffer = append(responseBuffer, []byte(response)...)
			}
			return responseBuffer, 0
		}

		switch args[0] {
		case "slabs":
			statsArray := []string{
				"STAT 1:chunk_size 96\r\n",
				"STAT 1:chunks_per_page 10922\r\n",
				"STAT 1:total_pages 1\r\n",
				"STAT 1:total_chunks 10922\r\n",
				"STAT 1:used_chunks 1\r\n",
				"STAT 1:free_chunks 10921\r\n",
				"STAT 1:free_chunks_end 0\r\n",
				"STAT 1:mem_requested 68\r\n",
				"STAT 1:get_hits 0\r\n",
				"STAT 1:cmd_set 0\r\n",
				"STAT 1:delete_hits 0\r\n",
				"STAT 1:incr_hits 0\r\n",
				"STAT 1:decr_hits 0\r\n",
				"STAT 1:cas_hits 0\r\n",
				"STAT 1:cas_badval 0\r\n",
				"STAT 1:touch_hits 0\r\n",
				"STAT 2:chunk_size 120\r\n",
				"STAT 2:chunks_per_page 8738\r\n",
				"STAT 2:total_pages 1\r\n",
				"STAT 2:total_chunks 8738\r\n",
				"STAT 2:used_chunks 0\r\n",
				"STAT 2:free_chunks 8738\r\n",
				"STAT 2:free_chunks_end 0\r\n",
				"STAT 2:mem_requested 0\r\n",
				"STAT 2:get_hits 0\r\n",
				"STAT 2:cmd_set 0\r\n",
				"STAT 2:delete_hits 0\r\n",
				"STAT 2:incr_hits 0\r\n",
				"STAT 2:decr_hits 0\r\n",
				"STAT 2:cas_hits 0\r\n",
				"STAT 2:cas_badval 0\r\n",
				"STAT 2:touch_hits 0\r\n",
				"STAT 39:chunk_size 524288\r\n",
				"STAT 39:chunks_per_page 2\r\n",
				"STAT 39:total_pages 1\r\n",
				"STAT 39:total_chunks 2\r\n",
				"STAT 39:used_chunks 0\r\n",
				"STAT 39:free_chunks 2\r\n",
				"STAT 39:free_chunks_end 0\r\n",
				"STAT 39:mem_requested 0\r\n",
				"STAT 39:get_hits 0\r\n",
				"STAT 39:cmd_set 0\r\n",
				"STAT 39:delete_hits 0\r\n",
				"STAT 39:incr_hits 0\r\n",
				"STAT 39:decr_hits 0\r\n",
				"STAT 39:cas_hits 0\r\n",
				"STAT 39:cas_badval 0\r\n",
				"STAT 39:touch_hits 0\r\n",
				"STAT active_slabs 3\r\n",
				"STAT total_malloced 3145728\r\n",
				"END\r\n",
			}
			responseBuffer := []byte{}
			for _, response := range statsArray {
				responseBuffer = append(responseBuffer, []byte(response)...)
			}
			return responseBuffer, 0
		case "items":
			statsArray := []string{
				fmt.Sprintf("STAT items:1:number %d\r\n", items),
				"STAT items:1:number_hot 0\r\n",
				"STAT items:1:number_warm 0\r\n",
				fmt.Sprintf("STAT items:1:number_cold %d\r\n", items),
				"STAT items:1:age_hot 0\r\n",
				"STAT items:1:age_warm 0\r\n",
				"STAT items:1:age 31\r\n",
				"STAT items:1:evicted 0\r\n",
				"STAT items:1:evicted_nonzero 0\r\n",
				"STAT items:1:evicted_time 0\r\n",
				"STAT items:1:outofmemory 0\r\n",
				"STAT items:1:tailrepairs 0\r\n",
				"STAT items:1:reclaimed 1\r\n",
				"STAT items:1:expired_unfetched 1",
				"STAT items:1:evicted_unfetched 0\r\n",
				"STAT items:1:evicted_active 0\r\n",
				"STAT items:1:crawler_reclaimed 0\r\n",
				"STAT items:1:crawler_items_checked 0\r\n",
				"STAT items:1:lrutail_reflocked 0\r\n",
				"STAT items:1:moves_to_cold 3\r\n",
				"STAT items:1:moves_to_warm 0\r\n",
				"STAT items:1:moves_within_lru 0\r\n",
				"STAT items:1:direct_reclaims 0\r\n",
				"STAT items:1:hits_to_hot 0\r\n",
				"STAT items:1:hits_to_warm 0\r\n",
				"STAT items:1:hits_to_cold 0\r\n",
				"STAT items:1:hits_to_temp 0\r\n",
				"END\r\n",
			}
			responseBuffer := []byte{}
			for _, response := range statsArray {
				responseBuffer = append(responseBuffer, []byte(response)...)
			}
			return responseBuffer, 0
		case "detail":
			return RESPONSE_OK, 0
		case "sizes":
			return append([]byte("STAT sizes_status disabled\r\n"), RESPONSE_END...), 0
		case "reset":
			return RESPONSE_RESET, 0
		default:
			break
		}

		return RESPONSE_ERROR, 0
	},
}

// TCP服务端连接
func tcpServer(address string, exitChan chan int) {
	l, err := net.Listen("tcp", address)

	if err != nil {
		fmt.Println(err.Error())
		exitChan <- 1
	}

	log.Println("[Memcache] Listning on " + address)

	defer l.Close()

	for {
		conn, err := l.Accept()
		trackID := randNumber(100000, 999999)

		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		go func() {
			skip := false
			reader := bufio.NewReader(conn)
			log.Printf("[Memcache %d] Accepted a client socket from %s\n", trackID, conn.RemoteAddr().String())
			for {
				str, err := reader.ReadString('\n')
				if skip {
					skip = false
					continue
				}
				if err != nil {
					log.Printf("[Memcache %d] Closed a client socket.", trackID)
					conn.Close()
					break
				}
				str = strings.TrimSpace(str)

				log.Printf("[Memcache %d] Client request: %s.", trackID, str)
				args := strings.Split(str, " ")
				function, exist := commands[args[0]]
				if !exist {
					conn.Write(RESPONSE_ERROR)
					continue
				}

				args = args[1:]

				for {
					response, requiredBytes := function(args)
					if requiredBytes == -1 {
						conn.Close()
						break
					}
					if requiredBytes == 0 {
						conn.Write(response)
						break
					}

					data := make([]byte, requiredBytes)
					_, err = io.ReadFull(reader, data)
					if err != nil {
						break
					}
					skip = true
					args = append(args, string(data))
				}

			}
		}()
	}

}

// func udpServer(address string, exitChan chan int) {
// 	udpAddr, err := net.ResolveUDPAddr("udp", address)
// 	if err != nil {
// 		fmt.Println(err.Error())
// 		exitChan <- 1
// 	}

// 	l, err := net.ListenUDP("udp", udpAddr)
// 	if err != nil {
// 		fmt.Println(err.Error())
// 		exitChan <- 1
// 	}

// 	go func() {
// 		buf := make([]byte, 1500)
// 		for {
// 			buf := make([]byte, 1500)
// 			plen, addr, _ := l.ReadFromUDP(buf)

// 		}
// 	}
// }()
// }

func Start(addr string) {
	// 创建一个程序结束码的通道
	exitChan := make(chan int)

	// 将服务器并发运行
	go tcpServer(addr, exitChan)
	// go udpServer(addr, exitChan)

	// 通道阻塞，等待接受返回值
	code := <-exitChan

	// 标记程序返回值并退出
	os.Exit(code)
}
