package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sync"
)

const (
	dataFile = "data.dat"
	crcFile  = "data.crc"
)

type object interface{}

type Interface interface {
	Serialize(filePath string) error
	AddEntry(key string, val interface{}, path ...string) bool
	FindEntries(recursive bool, path ...string) []interface{}
	DeletePath(path ...string) bool
	DeleteEntry(key string, path ...string) bool
}

type treeNode struct {
	ChildNodes map[string]*treeNode
	Entries    map[string]interface{}
	m          *sync.RWMutex
}

func NewTreeNode() Interface {
	return &treeNode{
		ChildNodes: make(map[string]*treeNode),
		Entries:    make(map[string]interface{}),
		m:          &sync.RWMutex{},
	}
}

func Deserialize(dir string) (Interface, error) {
	b, err := ioutil.ReadFile(path.Join(dir, dataFile))
	if err != nil {
		return nil, err
	}
	var hash []byte
	hash, err = ioutil.ReadFile(path.Join(dir, crcFile))
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(hash, getHash(b)) {
		return nil, fmt.Errorf("Checksum failed")
	}

	var tn treeNode
	err = json.Unmarshal(b, &tn)
	if err != nil {
		return nil, err
	}
	tn.m = &sync.RWMutex{}
	return &tn, nil
}

func (tn *treeNode) Serialize(dir string) error {
	if err := ensureDir(dir, os.FileMode(0755)); err != nil {
		return err
	}
	tn.m.RLock()
	defer tn.m.RUnlock()
	b, err := json.Marshal(tn)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(path.Join(dir, dataFile), b, 0644); err != nil {
		return err
	}
	if err := ioutil.WriteFile(path.Join(dir, crcFile), getHash(b), 0644); err != nil {
		return err
	}
	return nil
}

func (tn *treeNode) AddEntry(key string, val interface{}, path ...string) bool {
	tn.m.Lock()
	defer tn.m.Unlock()
	node := tn.ensureChildNode(path...)
	if node == nil {
		return false
	}
	node.Entries[key] = val
	return true
}

func (tn *treeNode) FindEntries(recursive bool, path ...string) []interface{} {
	tn.m.RLock()
	defer tn.m.RUnlock()
	childNode := tn.findChildNode(path...)
	if childNode == nil {
		return nil
	}

	retval := [][]interface{}{{}}
	childNode.findEntries(recursive, retval)
	return retval[0]
}

func (tn *treeNode) DeletePath(path ...string) bool {
	if len(path) == 0 {
		return false
	}
	tn.m.Lock()
	defer tn.m.Unlock()
	if parentNode := tn.findChildNode(path[:len(path)-1]...); parentNode != nil {
		if _, ok := parentNode.ChildNodes[path[len(path)-1]]; ok {
			delete(parentNode.ChildNodes, path[len(path)-1])
			return true
		}
	}
	return false
}

func (tn *treeNode) DeleteEntry(key string, path ...string) bool {
	tn.m.Lock()
	defer tn.m.Unlock()
	childNode := tn.findChildNode(path...)
	if childNode == nil {
		return false
	}
	if _, ok := childNode.Entries[key]; ok {
		delete(childNode.Entries, key)
		return true
	}
	return false
}

func (tn *treeNode) findEntries(recursive bool, ref [][]interface{}) {
	for _, entry := range tn.Entries {
		ref[0] = append(ref[0], entry)
	}
	if recursive {
		for _, node := range tn.ChildNodes {
			node.findEntries(recursive, ref)
		}
	}
}

func (tn *treeNode) findChildNode(path ...string) *treeNode {
	childNode := tn
	for _, subpath := range path {
		childNode = childNode.ChildNodes[subpath]
		if childNode == nil {
			return childNode
		}
	}
	return childNode
}

func (tn *treeNode) ensureChildNode(path ...string) *treeNode {
	childNode := tn
	for _, subpath := range path {
		newNode := childNode.ChildNodes[subpath]
		if newNode == nil {
			newNode = NewTreeNode().(*treeNode)
			childNode.ChildNodes[subpath] = newNode
		}
		childNode = newNode
	}
	return childNode
}

func ensureDir(path string, perm os.FileMode) error {
	s, err := os.Stat(path)
	if err != nil || !s.IsDir() {
		return os.Mkdir(path, perm)
	}
	return nil
}

func getHash(b []byte) []byte {
	h := md5.New()
	h.Write(b)
	return []byte(fmt.Sprintf("%x", h.Sum(nil)))
}

func main() {
	root := NewTreeNode()
	fmt.Println("Adding Entries")
	root.AddEntry("k", "v")
	root.AddEntry("foo", "bar", "local")
	root.AddEntry("foo1", "bar1", "local", "cluster")

	fmt.Println("Fetching Entries")
	for _, entry := range root.FindEntries(true, "local") {
		fmt.Printf("%s\n", entry)
	}

	fmt.Println("Serializing")
	if err := root.Serialize("./foo"); err != nil {
		fmt.Printf("Serialization Error:  %v,\n", err)
		return
	}

	fmt.Println("Deserializing")
	tn, err := Deserialize("./foo")
	if err != nil {
		fmt.Printf("Deserialization Error: %v\n", err)
		return
	}

	fmt.Println("Fetching Entries")
	for _, entry := range tn.FindEntries(true, "local") {
		fmt.Printf("%s\n", entry)
	}
}
