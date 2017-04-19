/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package container

import (
	"io/ioutil"
	"os"
	"time"
)

// OSInterface collects system level operations that need to be mocked out
// during tests.
type OSInterface interface {
	MkdirAll(path string, perm os.FileMode) error
	Symlink(oldname string, newname string) error
	Stat(path string) (os.FileInfo, error)
	Remove(path string) error
	Create(path string) (*os.File, error)
	Hostname() (name string, err error)
	Chtimes(path string, atime time.Time, mtime time.Time) error
	Pipe() (r *os.File, w *os.File, err error)
	ReadDir(dirname string) ([]os.FileInfo, error)
}

// RealOS is used to dispatch the real system level operaitons.
type RealOS struct{}

// MkDir will will call os.Mkdir to create a directory.
func (RealOS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Symlink will call os.Symlink to create a symbolic link.
func (RealOS) Symlink(oldname string, newname string) error {
	return os.Symlink(oldname, newname)
}

// Stat will call os.Stat to get the FileInfo for a given path
func (RealOS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// Remove will call os.Remove to remove the path.
func (RealOS) Remove(path string) error {
	return os.Remove(path)
}

// Create will call os.Create to create and return a file
// at path.
func (RealOS) Create(path string) (*os.File, error) {
	return os.Create(path)
}

// Hostname will call os.Hostname to return the hostname.
func (RealOS) Hostname() (name string, err error) {
	return os.Hostname()
}

// Chtimes will call os.Chtimes to change the atime and mtime of the path
func (RealOS) Chtimes(path string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(path, atime, mtime)
}

// Pipe will call os.Pipe to return a connected pair of pipe.
func (RealOS) Pipe() (r *os.File, w *os.File, err error) {
	return os.Pipe()
}

// ReadDir will call ioutil.ReadDir to return the files under the directory.
func (RealOS) ReadDir(dirname string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(dirname)
}
