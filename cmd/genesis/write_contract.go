// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AsimovNetwork/asimov/chaincfg"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/crypto"
	"github.com/AsimovNetwork/asimov/vm/fvm"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"strings"
)


type compiledContract struct {
	name     string
	byteCode string
	abi      string
	// constructorCode string
	initCode     string
	address      string
	delegateAddr string
}

func writeSystemContract(systemContractFolder, network string, skipPoa bool) {
	contracts := getFiles(systemContractFolder)

	err := os.Chdir("cmd/genesis")
	if err != nil {
		panic(err)
	}

	contractSlice := make([]*compiledContract, 0)
	// compile contracts
	for _, contract := range contracts {
		if skipPoa {
			_, file := path.Split(contract)
			if strings.Contains(strings.ToUpper(file), "POA") {
				continue
			}
		}
		contractName, byteCode, abi, delegateAddr := compile(contract)
		contractSlice = append(contractSlice, &compiledContract{
			name:         contractName,
			byteCode:     byteCode,
			abi:          abi,
			initCode:     "",
			delegateAddr: delegateAddr,
		})
	}

	writeContractsName(contractSlice)
	writeContracts(contractSlice, network, skipPoa)

	err = os.Chdir("../../")
	if err != nil {
		panic(err)
	}
}

func writeContractsName(contractSlice []*compiledContract) {
	file, err := os.Create("../../common/genesis_contracts.go")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	imports := `// Code generated by github.com/AsimovNetwork/asimov/cmd/genesis/write_contract. DO NOT EDIT.

package common
`
	writeBytes(file, []byte(imports))

	codemap := `
// Map of ContractCode values back to their constant names for pretty printing.
var ContractCodeStrings = map[Address]string{
`
	writeBytes(file, []byte(codemap))
	pairformat := "	%s : \"%s\",\n"
	for _, v := range contractSlice {
		writeBytes(file, []byte(fmt.Sprintf(pairformat, v.name, v.name)))
	}
	writeBytes(file, []byte("}\n"))


	structType := `

// Contract %s Definition`

	function := `

func Contract%s_%sFunction() (string) {
	return %q
}`
	for _, v := range contractSlice {
		writeBytes(file, []byte(fmt.Sprintf(structType, v.name)))
		abiSlice := make([]map[string]interface{}, 0)
		err := json.Unmarshal([]byte(v.abi), &abiSlice)
		if err != nil {
			panic(err)
		}
		for _, abi := range abiSlice {
			abiType, ok := abi["type"]
			if ok && abiType == "function" {
				funcName, ok := abi["name"]
				if ok {
					writeBytes(file, []byte(fmt.Sprintf(function, v.name, firstToUpper(funcName.(string)), funcName.(string))))
				}
			}
		}
	}

}

func writeContracts(contractSlice []*compiledContract, network string, skipPoa bool) {
	file, err := os.Create("contract.go")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	imports := `// Code generated by github.com/AsimovNetwork/asimov/cmd/genesis/write_contract. DO NOT EDIT.

package main

import (
	"github.com/AsimovNetwork/asimov/chaincfg"
	"github.com/AsimovNetwork/asimov/common"
)

`
	writeBytes(file, []byte(imports))

	for _, v := range contractSlice {
		abiSlice := make([]map[string]interface{}, 0)
		err := json.Unmarshal([]byte(v.abi), &abiSlice)
		if err != nil {
			panic(err)
		}
		initArgSlice := make([]interface{}, 0)
		hasInitFunction := false
		for _, abi := range abiSlice {
			abiType, _ := abi["type"]
			abiName, _ := abi["name"]
			if abiType == "function" && abiName == "init" {
				hasInitFunction = true
				initArgs, _ := (abi["inputs"]).([]interface{})
				for _, initArg := range initArgs {
					argName, _ := initArg.(map[string]interface{})["name"]
					val, ok := chaincfg.NetConstructorArgsMap[network][argName.(string)]
					if !ok {
						panic(errors.New("init function arg not found, argName : " + argName.(string)))
					}
					initArgSlice = append(initArgSlice, val)
				}
				break
			}
		}

		if hasInitFunction {
			initCode, err := fvm.PackFunctionArgs(v.abi, "init", initArgSlice...)
			if err != nil {
				panic(err)
			}
			v.initCode = common.Bytes2Hex(initCode)
		}

		// pre-build addresses of current contracts and write them as parameters
		byteCode := common.Hex2Bytes(v.byteCode)
		currentAddress, err := crypto.CreateContractAddress(chaincfg.OfficialAddress[:], nil, byteCode)
		if err != nil {
			panic(err)
		}
		v.address = currentAddress.String()
	}

	deployedTemplateContract := `var deployedTemplateContract = map[common.Address][]chaincfg.ContractInfo{
`
	writeBytes(file, []byte(deployedTemplateContract))
	for _, v := range contractSlice {
		temp := `	common.%s: {{
		Name:	 %q,
		Code:    "%s",
		AbiInfo: "%s",
		InitCode: "%s",
		Address: common.HexToAddress(%q).Bytes(),
		BlockHeight: 0,
	}},`
		writeBytes(file, []byte(fmt.Sprintf(temp, v.name, v.name, v.byteCode, strings.Replace(v.abi, "\"", "\\\"", -1), v.initCode, v.address[2:])))
		writeBytes(file, []byte("\n"))
	}
	writeBytes(file, []byte("}"))
}

func firstToUpper(str string) string {
	upper := strings.ToUpper(string(str[0]))
	return upper + str[1:]
}

func writeBytes(w io.Writer, b []byte) {
	_, err := w.Write(b)
	if err != nil {
		panic(err)
	}
}

func getFiles(path string) []string {
	files := make([]string, 0)
	fileList, err := ioutil.ReadDir(path)
	if err != nil {
		panic(err)
	}
	for _, file := range fileList {
		if !file.IsDir() {
			files = append(files, path + "/" + file.Name())
		}
	}
	return files
}

func parsingDelegateAddr(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}

	defer f.Close()
	reader := bufio.NewReader(f)
	var delegateAddr = ""
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				return delegateAddr, errors.New(fmt.Sprintf("%s's delegateAddr not found", filePath))
			}
			return delegateAddr, err
		}

		if ok, _ := regexp.Match("delegateAddr", line); ok {
			tmp := strings.Trim(strings.Split(string(line), "=")[1], " ")
			delegateAddr = tmp[:len(tmp)-1]
			if !strings.HasPrefix(delegateAddr, "0x63") {
				return "", errors.New(fmt.Sprintf("%s's delegateAddr should start with 0x63", filePath))
			}
			return delegateAddr, nil
		}
	}
}

func compile(path string) (string, string, string, string) {
	os.Mkdir("temp", os.ModePerm)
	os.Chdir("temp")
	os.Mkdir("library", os.ModePerm)

	// copy the files to be compiled to a temporary directory
	fileName := copySource(path)

	/// copy the system contract dependent files to the temp directory
	os.Chdir("library")
	copyImport()

	// parsing the delegate address
	os.Chdir("..")
	delegateAddr, err := parsingDelegateAddr(fileName)
	if err != nil {
		panic(err)
	}

	// compile file by solc compiler
	contractName, byteCode, abi := callSolc(fileName)

	// delete temporary folder
	os.Chdir("../")
	os.RemoveAll("temp")

	return contractName, byteCode, abi, delegateAddr
}

func copySource(source string) string {
	src, err := os.Open(source)
	if err != nil {
		panic(err)
	}
	defer src.Close()
	sourceArray := strings.Split(source, "/")
	fileName := sourceArray[len(sourceArray)-1]
	dst, err := os.Create(fileName)
	if err != nil {
		panic(err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		panic(err)
	}
	return fileName
}

func copyImport() {
	curDir, _ := os.Getwd()
	homeDir := strings.Replace(curDir, "/cmd/genesis/temp/library", "", -1)
	sourceDir := homeDir + "/systemcontracts/files/library"

	filepathNames := getFiles(sourceDir)
	for _, url := range filepathNames {
		copySource(url)
	}
}

// return byte code & abi
func callSolc(fileName string) (string, string, string) {
	var dataCmd *exec.Cmd
	// compile bin
	sysType := runtime.GOOS
	if sysType == "darwin" {
		fmt.Println("../solc --bin --abi ", fileName)
		dataCmd = exec.Command("../solc", "--bin", "--abi", fileName)
	}
	if sysType == "linux" {
		fmt.Println("../solc_linux --bin --abi ", fileName)
		dataCmd = exec.Command("../solc_linux", "--bin", "--abi", fileName)
	}
	if dataCmd == nil {
		panic(errors.New("unsupported system type to compile system contracts"))
	}

	dataOut, err := dataCmd.Output()
	if err != nil {
		panic(err)
	}
	// fmt.Println(string(dataOut))
	contractName, byteCode, abi := parsingResult(dataOut, fileName)
	if contractName == "" || byteCode == "" || abi == "" {
		panic(errors.New("compile err"))
	}

	return contractName, byteCode, abi
}

// parsingResult parses from source output byte code
// and returns contract name, byte code and abi
func parsingResult(result []byte, fileName string) (string, string, string) {
	buf := bufio.NewReader(bytes.NewReader(result))
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			} else {
				panic(err)
			}
		}
		line = strings.TrimSpace(line)
		if strings.Contains(line, fileName) {
			// Binary:
			_, err := buf.ReadString('\n')
			if err != nil {
				panic(err)
			}
			// byte code
			byteCode, err := buf.ReadString('\n')
			if err != nil {
				panic(err)
			}
			byteCodeStr := strings.TrimSpace(byteCode)
			if byteCodeStr != "" {
				// Contract JSON ABI
				_, err := buf.ReadString('\n')
				if err != nil {
					panic(err)
				}

				// abi
				abi, err := buf.ReadString('\n')
				if err != nil {
					panic(err)
				}
				abiStr := strings.TrimSpace(abi)

				contractName := strings.Split(strings.Split(line, ":")[1], " ")[0]

				return contractName, byteCodeStr, abiStr
			}
		}
	}
	return "", "", ""
}
