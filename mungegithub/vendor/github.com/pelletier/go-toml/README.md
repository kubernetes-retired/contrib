# go-toml

Go library for the [TOML](https://github.com/mojombo/toml) format.

This library supports TOML version
[v0.4.0](https://github.com/toml-lang/toml/blob/master/versions/en/toml-v0.4.0.md)

[![GoDoc](https://godoc.org/github.com/pelletier/go-toml?status.svg)](http://godoc.org/github.com/pelletier/go-toml)
[![Build Status](https://travis-ci.org/pelletier/go-toml.svg?branch=master)](https://travis-ci.org/pelletier/go-toml)
[![Coverage Status](https://coveralls.io/repos/github/pelletier/go-toml/badge.svg?branch=master)](https://coveralls.io/github/pelletier/go-toml?branch=master)

## Features

Go-toml provides the following features for using data parsed from TOML documents:

* Load TOML documents from files and string data
* Easily navigate TOML structure using TomlTree
* Line & column position data for all parsed elements
* Query support similar to JSON-Path
* Syntax errors contain line and column numbers

Go-toml is designed to help cover use-cases not covered by reflection-based TOML parsing:

* Semantic evaluation of parsed TOML
* Informing a user of mistakes in the source document, after it has been parsed
* Programatic handling of default values on a case-by-case basis
* Using a TOML document as a flexible data-store

## Import

    import "github.com/pelletier/go-toml"

## Usage

### Example

Say you have a TOML file that looks like this:

```toml
[postgres]
user = "pelletier"
password = "mypassword"
```

Read the username and password like this:

```go
import (
    "fmt"
    "github.com/pelletier/go-toml"
)

config, err := toml.LoadFile("config.toml")
if err != nil {
    fmt.Println("Error ", err.Error())
} else {
    // retrieve data directly
    user := config.Get("postgres.user").(string)
    password := config.Get("postgres.password").(string)

    // or using an intermediate object
    configTree := config.Get("postgres").(*toml.TomlTree)
    user = configTree.Get("user").(string)
    password = configTree.Get("password").(string)
    fmt.Println("User is ", user, ". Password is ", password)

    // show where elements are in the file
    fmt.Println("User position: %v", configTree.GetPosition("user"))
    fmt.Println("Password position: %v", configTree.GetPosition("password"))

    // use a query to gather elements without walking the tree
    results, _ := config.Query("$..[user,password]")
    for ii, item := range results.Values() {
      fmt.Println("Query result %d: %v", ii, item)
    }
}
```

## Documentation

The documentation and additional examples are available at
[godoc.org](http://godoc.org/github.com/pelletier/go-toml).

## Contribute

Feel free to report bugs and patches using GitHub's pull requests system on
[pelletier/go-toml](https://github.com/pelletier/go-toml). Any feedback would be
much appreciated!

### Run tests

You have to make sure two kind of tests run:

1. The Go unit tests
2. The TOML examples base

You can run both of them using `./test.sh`.

## License

Copyright (c) 2013 - 2016 Thomas Pelletier, Eric Anderton

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is furnished to do
so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
