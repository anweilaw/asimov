package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

type BlockChainInfoResponse struct {
	Chain                string  `json:"chain"`
	Blocks               int32   `json:"blocks"`
	BestBlockHash        string  `json:"bestblockhash"`
	MedianTime           int64   `json:"mediantime"`
	VerificationProgress float64 `json:"verificationprogress,omitempty"`
	Pruned               bool    `json:"pruned"`
	PruneHeight          int32   `json:"pruneheight,omitempty"`
}

type blockChainInfoJson struct {
	Result BlockChainInfoResponse `json:"result"`
}

func BytesToStringFast(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

type Rpcrequest struct {
	Id      uint32  `json:"id"`
	Jsonrpc string  `json:"jsonrpc"`
	Method  string  `json:"method"`
	Params  []int32 `json:"params"`
}

type ValidatorTx struct {
	Txid        string   `json:"txid"`
	Vin         []string `json:"vin"`
	OutAddress  string   `json:"outAddress"`
	AddressType int32    `json:"addressType"`
}

type MinersResponse struct {
	Height       int32         `json:"height"`
	Pool         string        `json:"pool"`
	Hash         string        `json:"hash"`
	Time         uint32        `json:"time"`
	Address      string        `json:"address"`
	ValidatorTxs []ValidatorTx `json:"validatorTxs"`
}

type blockminerJson struct {
	Result []MinersResponse `json:"result"`
}

type mapFromTest struct {
	Vin        string `json:"vin"`
	OutAddress string `json:"outAddress"`
}

// CACHED DATA
var templateInfo = MinersResponse{
	Height:  0,
	Pool:    "AsAutoTest",
	Hash:    "0000000000000000000000000000000000000000000000000000000000000000",
	Time:    1579083005,
	Address: "",
	// empty txs
	ValidatorTxs: []ValidatorTx{},
}
var mappingInfo = templateInfo
var infoMutex sync.Mutex

// Store some specific height block info to report to different vvs-core.
var cachedBlockInfo = make(map[int]MinersResponse, 1000)
var minerCandidates = []string{"InitAddress"}
var bRefreshing = false

var dataCh = make(chan mapFromTest, 20)

// END CACHED DATA

var debugLog *log.Logger

// Response back to vvs
func minersToNode(w http.ResponseWriter, req *http.Request) {
	body, _ := ioutil.ReadAll(req.Body)
	debugLog.Println("vvs request", string(body))

	result := Rpcrequest{}
	json.Unmarshal([]byte(body), &result)

	var responseData interface{}
	if result.Method == "getblockchaininfo" {
		responseData = blockChainInfoJson{
			BlockChainInfoResponse{
				Chain:                "develop",
				Blocks:               400,
				BestBlockHash:        "0000000012345678123456781234567812345678123456781234567812345678",
				MedianTime:           1523456678,
				VerificationProgress: 1,
				Pruned:               false,
				PruneHeight:          0,
			},
		}
	} else if result.Method == "listblockminerinfo" {
		heightStart := int(result.Params[0])
		count := int(result.Params[1])
		debugLog.Printf(
			"VVS ask for height start from %d, count: %d",
			heightStart, count)

		tmpResult := transactionData(heightStart, count)

		responseData = blockminerJson{
			tmpResult,
		}
	}

	var str string
	if ba, e := json.Marshal(responseData); e != nil {
		debugLog.Println("json.Marshal failed:", e)
	} else {
		str = BytesToStringFast(ba)
	}
	fmt.Fprintln(w, str)
	fmt.Print(str)
}

func transactionData(heightStart, count int) []MinersResponse {
	tmpResult := make([]MinersResponse, count)
	// a large lock
	infoMutex.Lock()
	defer infoMutex.Unlock()

	// and get from cache
	if _, ok := cachedBlockInfo[heightStart]; ok {
		debugLog.Printf("Get cached block info start from %d", heightStart)
		for i := heightStart; i < heightStart+count; i++ {
			tmpResult[i-heightStart] = cachedBlockInfo[i]
		}
		return tmpResult
	}

	// else
	for i := 0; i < count; i++ {
		tmpResult[i] = templateInfo
	}

LOOPCH:
	for {
		select {
		case mapData := <-dataCh:
			if !bRefreshing {
				// clear first
				minerCandidates = minerCandidates[:0]
				bRefreshing = true
			}
			fmt.Print(json.Marshal(mapData))
			minerCandidates = append(minerCandidates, mapData.Vin)

			tx := ValidatorTx{
				Txid:        "testTxid",
				Vin:         []string{mapData.Vin},
				OutAddress:  mapData.OutAddress,
				AddressType: 1,
			}
			mappingInfo.ValidatorTxs = append(
				mappingInfo.ValidatorTxs,
				tx,
			)
		default:
			bRefreshing = false
			break LOOPCH
		}
	}

	// assign height, miner address parameters
	minerLength := len(minerCandidates)
	if minerLength > count {
		debugLog.Println("WARNING: the number of miner candidates is more than asking count of blocks!")
	}
	for m := 0; m < count; m++ {
		if heightStart + m == 145{
			tmpResult[m] = mappingInfo
		}
		tmpResult[m].Height = int32(heightStart + m)
		// divide averagely
		tmpResult[m].Address = minerCandidates[m%minerLength]
	}

	debugLog.Printf("Generate and store new info of blocks start from %d, count %d", heightStart, count)
	// store
	for _, res := range tmpResult {
		cachedBlockInfo[int(res.Height)] = res
	}

	return tmpResult
}

// Store MinersInfo from the request of AsAutoTest
func storeBitcoinMinerInfo(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	debugLog.Println("Recv:", string(body))
	result := []mapFromTest{}
	if err := json.Unmarshal(body, &result); err != nil {
		debugLog.Fatal(err)
	}
	fmt.Print(body)
	for _, m := range result {
		dataCh <- m
	}
}

func main() {
	pathstr := "/tmp/fakeBitcoin"
	if r := os.MkdirAll(pathstr, os.ModePerm); r != nil {
		log.Panicln("MkdirAll failed", r)
	}
	logFileName := filepath.Join(pathstr, fmt.Sprintf("%d.log", os.Getpid()))
	logFile, err := os.Create(logFileName)
	defer logFile.Close()
	if err != nil {
		log.Fatalln("Open log file error!")
	}
	debugLog = log.New(logFile, "", log.LstdFlags|log.Lshortfile)

	//设置路由和接收HTTP请求的方法
	mux := http.NewServeMux()
	mux.HandleFunc("/storeMinersInfo", storeBitcoinMinerInfo)
	mux.HandleFunc("/", minersToNode)

	//设置http服务，默认端口8000
	var port string
	flag.StringVar(&port, "p", "8000", "Indicate listen port")
	flag.Parse()
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	debugLog.Printf("fakeBitcoin start successfully, at port[%s].\n", port)
	//启动监听
	if e := server.ListenAndServe(); e != nil {
		debugLog.Panicln(e)
	}
}
