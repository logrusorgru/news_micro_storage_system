News Micro Storage System
=========================

[![GoDoc](https://godoc.org/github.com/logrusorgru/news_micro_storage_system?status.svg)](https://godoc.org/github.com/logrusorgru/news_micro_storage_system)
[![WTFPL License](https://img.shields.io/badge/license-wtfpl-blue.svg)](http://www.wtfpl.net/about/)
[![Build Status](https://travis-ci.org/logrusorgru/news_micro_storage_system.svg)](https://travis-ci.org/logrusorgru/news_micro_storage_system)
[![Coverage Status](https://coveralls.io/repos/logrusorgru/news_micro_storage_system/badge.svg?branch=master)](https://coveralls.io/r/logrusorgru/news_micro_storage_system?branch=master)
[![GoReportCard](https://goreportcard.com/badge/logrusorgru/news_micro_storage_system)](https://goreportcard.com/report/logrusorgru/news_micro_storage_system)

The _news_micro_storage_system_ is an application to obtain and store news
based on cockroackdb and uses bicroservice architecture.

# Representation

```
blah-blah
```

# Get

#### Get

```
go get -u github.com/logrusorgru/news_micro_storage_system
```

#### Generate

Regenerate protbuf messages if you want

###### Prepare


```
go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
```

###### Generate

(TODO: go generate)

```
cd $GOPATH/github.com/logrusorgru/news_micro_storage_system
protoc --go_out=:. ./msg/*.proto
```

#### Test

Prepare CockroachDB for tests. Feel free to choose your DB address and port.

```
cockroach start --insecure --listen-addr=localhost
```

Open Cockroach SQL console

```
cockroach sql --insecure
```

And create test database and user. Feel free to choose you DB name/user.

```
CREATE DATABASE test_news_items;
CREATE USER test_news_items;
GRANT ALL ON DATABASE test_news_items TO test_news_items;
```

Prepare NATS for tets

```
nats-server
```

Then test

```
go test -cover -race github.com/logrusorgru/news_micro_storage_system/... \
    -test-db-addr=localhost            \
    -test-db-port=26257                \
    -test-db-name=test_news_items      \
    -test-db-user=test_news_items      \
    -test-nats-subject=test_news_items
```

or just

```
go test -cover -race github.com/logrusorgru/news_micro_storage_system/...
```

if the names are as above.

# Start

### CockroachDB

Start CockroachDB server, or make sure it's running.

```
cockroach start --insecure --listen-addr=localhost
```

//


# Licensing

Copyright © 2019 Konstantin Ivanov <kostyarin.ivanov@gmail.com>  
This work is free. You can redistribute it and/or modify it under the
terms of the Do What The Fuck You Want To Public License, Version 2,
as published by Sam Hocevar. See the LICENSE file for more details.
