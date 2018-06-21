package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

const (
	PakVersion  = 5
	PakEncoding = 1
)

type PakHeader struct {
	Version       uint32
	Encodeing     uint32
	ResourceCount uint16
	AliasCount    uint16
}

type PakEntryRaw struct {
	ResourceId uint16
	FileOffset uint32
}

type PakEntry struct {
	ResourceId     uint16
	FileOffset     uint32
	NextResourceId uint16
	NextFileOffset uint32
}

type PakAlias struct {
	ResourceId uint16
	EntryIndex uint16
}

func SHA1(data []byte) string {
	a := sha1.Sum(data)
	return hex.EncodeToString(a[:])
}

type EntryNode struct {
	ResourceId uint16 `json:"id"`
	Path       string `json:"path"`
	// Gzip       bool   `json:"gzip"`
}

type AliasNode struct {
	ResourceId uint16 `json:"id"`
	EntryIndex uint16 `json:"index"`
}

type MetaRecord struct {
	Entry []EntryNode `json:"entry"`
	Alias []AliasNode `json:"alias"`
}

type LangNode struct {
	ResourceId uint16 `json:"id"`
	Text       string `json:"text"`
	// Gzip       bool   `json:"gzip"`
}
type LangRecord struct {
	Entry []LangNode  `json:"entry"`
	Alias []AliasNode `json:"alias"`
}

func ToJson(data interface{}) []byte {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "\t")
	_ = encoder.Encode(data)
	return buf.Bytes()
}

func SaveToFile(path string, buf []byte) {
	os.MkdirAll(filepath.Dir(path), os.ModePerm)
	ioutil.WriteFile(path, []byte(buf), os.ModePerm)
}

func unpack(path string) {

	name := filepath.Ext(path)
	name = path[0 : len(path)-len(name)]

	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// 检查文件头
	var h PakHeader
	err = binary.Read(f, binary.LittleEndian, &h)
	if h.Version != PakVersion || h.Encodeing != PakEncoding {
		log.Fatal("invalid pak file")
	}
	// fmt.Printf("%+v\n", h)

	// 加载识别数据库
	db, err := Asset("assets/res_sha1.json")
	var json_db map[string]string
	json.Unmarshal(db, &json_db)

	write_files := make(map[string]bool)

	var mr MetaRecord

	var r PakEntryRaw
	for i := 0; i < int(h.ResourceCount); i++ {
		// 读取索引
		f.Seek(int64(binary.Size(h)+i*binary.Size(r)), io.SeekStart)

		var e PakEntry
		binary.Read(f, binary.LittleEndian, &e)

		// 读取内容
		f.Seek(int64(e.FileOffset), io.SeekStart)

		data := make([]byte, e.NextFileOffset-e.FileOffset)
		f.Read(data)

		// 自动判断名称
		path, err := json_db[SHA1(data)]
		if !err {
			path = fmt.Sprintf("unknown/%d", e.ResourceId)
		}

		// 检查重复名称
		_, ok := write_files[path]
		if ok {
			path = fmt.Sprintf("unknown/%d", e.ResourceId)
		}
		write_files[path] = true

		// 写入文件
		fmt.Println("write " + path)
		path = name + "/" + path
		os.MkdirAll(filepath.Dir(path), os.ModePerm)
		ioutil.WriteFile(path, data, os.ModePerm)

		en := EntryNode{e.ResourceId, path}
		mr.Entry = append(mr.Entry, en)
	}

	for i := 0; i < int(h.AliasCount); i++ {
		var a PakAlias

		// 读取索引
		f.Seek(int64(binary.Size(h)+int(h.ResourceCount+1)*binary.Size(r)+i*binary.Size(a)), io.SeekStart)

		binary.Read(f, binary.LittleEndian, &a)

		an := AliasNode{a.ResourceId, a.EntryIndex}
		mr.Alias = append(mr.Alias, an)
	}

	output := name + ".json"
	SaveToFile(output, ToJson(mr))
	fmt.Printf("\nwrite %s ok\n", output)
}

func ReadFromFile(path string) []byte {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	return data
}

func GetFileSize(path string) uint32 {
	fi, err := os.Stat(path)
	if err != nil {
		log.Fatal(err)
	}
	return uint32(fi.Size())
}

func repack(path string) {
	result := MetaRecord{}
	err := json.Unmarshal(ReadFromFile(path), &result)
	if err != nil {
		fmt.Println(err)
		return
	}

	name := filepath.Ext(path)
	name = path[0 : len(path)-len(name)]

	output := name + ".pak"
	f, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// 写入文件头
	var h PakHeader
	h.Version = PakVersion
	h.Encodeing = PakEncoding
	h.ResourceCount = uint16(len(result.Entry))
	h.AliasCount = uint16(len(result.Alias))
	binary.Write(f, binary.LittleEndian, &h)

	var r PakEntryRaw
	var a PakAlias

	//写入entry
	var offset = uint32(binary.Size(h)) + uint32(h.ResourceCount+1)*uint32(binary.Size(r)) + uint32(h.AliasCount)*uint32(binary.Size(a))
	for _, v := range result.Entry {
		r.ResourceId = v.ResourceId
		r.FileOffset = offset
		offset += GetFileSize(v.Path)
		binary.Write(f, binary.LittleEndian, &r)
		// fmt.Printf("%+v %+v  %+v \n", i, offset, GetFileSize(v.Path))
	}

	//写入entry末尾
	r.ResourceId = 0
	r.FileOffset = offset
	binary.Write(f, binary.LittleEndian, &r)

	//写入alias
	for _, v := range result.Alias {
		a.ResourceId = v.ResourceId
		a.EntryIndex = v.EntryIndex
		binary.Write(f, binary.LittleEndian, &a)
		// fmt.Printf("%+v %+v\n", a.ResourceId, a.EntryIndex)
	}

	//写入文件内容
	for _, v := range result.Entry {
		f.Write(ReadFromFile(v.Path))
	}

	fmt.Printf("repack %s ok\n", output)
}

func lang_unpack(path string) {

	name := filepath.Ext(path)
	name = path[0 : len(path)-len(name)]

	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// 检查文件头
	var h PakHeader
	err = binary.Read(f, binary.LittleEndian, &h)
	if h.Version != PakVersion || h.Encodeing != PakEncoding {
		log.Fatal("invalid pak file")
	}

	var lr LangRecord

	var r PakEntryRaw
	for i := 0; i < int(h.ResourceCount); i++ {
		// 读取索引
		f.Seek(int64(binary.Size(h)+i*binary.Size(r)), io.SeekStart)

		var e PakEntry
		binary.Read(f, binary.LittleEndian, &e)

		// 读取内容
		f.Seek(int64(e.FileOffset), io.SeekStart)

		data := make([]byte, e.NextFileOffset-e.FileOffset)
		f.Read(data)

		en := LangNode{e.ResourceId, string(data)}
		lr.Entry = append(lr.Entry, en)
	}

	for i := 0; i < int(h.AliasCount); i++ {
		var a PakAlias

		// 读取索引
		f.Seek(int64(binary.Size(h)+int(h.ResourceCount+1)*binary.Size(r)+i*binary.Size(a)), io.SeekStart)

		binary.Read(f, binary.LittleEndian, &a)

		an := AliasNode{a.ResourceId, a.EntryIndex}
		lr.Alias = append(lr.Alias, an)
	}

	output := name + ".json"
	SaveToFile(output, ToJson(lr))
	fmt.Printf("write %s ok\n", output)
}

func lang_repack(path string) {
	result := LangRecord{}
	err := json.Unmarshal(ReadFromFile(path), &result)
	if err != nil {
		fmt.Println(err)
		return
	}

	name := filepath.Ext(path)
	name = path[0 : len(path)-len(name)]

	output := name + ".pak"
	f, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// 写入文件头
	var h PakHeader
	h.Version = PakVersion
	h.Encodeing = PakEncoding
	h.ResourceCount = uint16(len(result.Entry))
	h.AliasCount = uint16(len(result.Alias))
	binary.Write(f, binary.LittleEndian, &h)

	var r PakEntryRaw
	var a PakAlias

	//写入entry
	var offset = uint32(binary.Size(h)) + uint32(h.ResourceCount+1)*uint32(binary.Size(r)) + uint32(h.AliasCount)*uint32(binary.Size(a))
	for _, v := range result.Entry {
		r.ResourceId = v.ResourceId
		r.FileOffset = offset
		offset += uint32(len(v.Text))
		binary.Write(f, binary.LittleEndian, &r)
		// fmt.Printf("%+v %+v  %+v \n", i, offset, GetFileSize(v.Path))
	}

	//写入entry末尾
	r.ResourceId = 0
	r.FileOffset = offset
	binary.Write(f, binary.LittleEndian, &r)

	//写入alias
	for _, v := range result.Alias {
		a.ResourceId = v.ResourceId
		a.EntryIndex = v.EntryIndex
		binary.Write(f, binary.LittleEndian, &a)
		// fmt.Printf("%+v %+v\n", a.ResourceId, a.EntryIndex)
	}

	//写入文件内容
	for _, v := range result.Entry {
		f.Write([]byte(v.Text))
	}

	fmt.Printf("repack %s ok\n", output)
}

func usage() {
	fmt.Println("usage:")
	fmt.Println("pak_tools -c=unpack -f=resources.pak")
	fmt.Println("\tunpack files in resources.pak to resources.json and resources folder")
	fmt.Println("pak_tools -c=repack -f=resources.json")
	fmt.Println("\trepack files to resources.pak according to resources.json")
	fmt.Println("pak_tools -c=lang_unpack -f=zh-CN.pak")
	fmt.Println("\textract the text in zh-CN.pak to zh-CN.json")
	fmt.Println("pak_tools -c=lang_repack -f=zh-CN.json")
	fmt.Println("\trepack text to zh-CN.pak from zh-CN.json")

	var input string
	fmt.Scanln(&input)
}

func main() {
	if len(os.Args) < 3 {
		usage()
		return
	}
	command := flag.String("c", "", "command")
	file := flag.String("f", "", "file path")
	flag.Parse()

	if *command == "unpack" {
		unpack(*file)
	} else if *command == "repack" {
		repack(*file)
	} else if *command == "lang_unpack" {
		lang_unpack(*file)
	} else if *command == "lang_repack" {
		lang_repack(*file)
	}
}
