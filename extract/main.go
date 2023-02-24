// Copyright 2018 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
    "flag"
    "fmt"
    "io"
    "net/url"
    "os"
    "path/filepath"
    "strings"

    "github.com/coreos/ignition/v2/config"
    "github.com/coreos/ignition/v2/internal/resource"
    "github.com/coreos/ignition/v2/internal/version"
)

var (
    flagVersion bool
    flagOutput  string
)

func init() {
    flag.BoolVar(&flagVersion, "version", false, "print the version of ignition-extract")
    flag.StringVar(&flagOutput, "output", "false", "empty path for output")
    flag.Usage = func() {
        fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n  %s [flags] config.ign\n\n", os.Args[0])
        flag.PrintDefaults()
    }
}

func main() {
    flag.Parse()

    runIgnExtract(flag.Args())
}

func stdout(format string, a ...interface{}) {
    fmt.Fprintf(os.Stdout, strings.TrimSpace(format)+"\n", a...)
}

func stderr(format string, a ...interface{}) {
    fmt.Fprintf(os.Stderr, strings.TrimSpace(format)+"\n", a...)
}

func die(format string, a ...interface{}) {
    stderr(format, a...)
    os.Exit(1)
}

func runIgnExtract(args []string) {
    if flagVersion {
        stdout(version.String)
        return
    }

    if len(args) != 1 {
        flag.Usage()
        os.Exit(1)
    }

    abs, err := filepath.Abs(flagOutput)
    if err != nil {
        die("output not valid: %s", err)
    }
    dir, err := os.ReadDir(abs)
    if err != nil {
        if os.IsNotExist(err) {
            stdout(fmt.Sprintf("output dir not found, creating dir at: %s", abs))
            err := os.Mkdir(abs, os.ModePerm)
            if err != nil {
                die("can't create output dir: %s: &v", abs, err)
            }
        } else {
            die("can't open output dir: %s: &v", abs, err)
        }
    }

    if len(dir) != 0 {
        die("output dir not empty: %s", abs)
    }

    var blob []byte
    if args[0] == "-" {
        blob, err = io.ReadAll(os.Stdin)
    } else {
        blob, err = os.ReadFile(args[0])
    }
    if err != nil {
        die("couldn't read config: %v", err)
    }
    cfg, rpt, err := config.Parse(blob)
    if len(rpt.Entries) > 0 {
        stdout(rpt.String())
    }
    if rpt.IsFatal() {
        os.Exit(1)
    }
    if err != nil {
        die("couldn't parse config: %v", err)
    }

    fetcher := resource.Fetcher{}

    for _, file := range cfg.Storage.Files {
        func() {
            if !*file.Overwrite {
                stdout(fmt.Sprintf("skiping non-overwrite file: %s", file.Path))
                return
            }
            filePath := filepath.Join(abs, file.Path)
            dir := filepath.Dir(filePath)
            err := os.MkdirAll(dir, os.ModePerm)
            if err != nil {
                stderr(fmt.Sprintf("cannot mkdir: %s: %v", dir, err))
                return
            }
            osFile, err := os.Create(filePath)
            if err != nil {
                stderr(fmt.Sprintf("cannot create file content: %s, %v", filePath, err))
                return
            }
            defer osFile.Close()

            fileUrl, err := url.Parse(*file.Contents.Source)
            if err != nil {
                stderr(fmt.Sprintf("cannot read content: %v", err))
                return
            }

            headers, err := file.Contents.HTTPHeaders.Parse()
            if err != nil {
                stderr(fmt.Sprintf("cannot read content headers: %v", err))
                return
            }

            stdout(file.Path)
            err = fetcher.Fetch(*fileUrl, osFile, resource.FetchOptions{
                Headers: headers,
                })
            if err != nil {
                stderr(fmt.Sprintf("cannot write file: %v", err))
                return
            }
        }()

    }
}
