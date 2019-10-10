News Micro Storage System
=========================

[![GoDoc](https://godoc.org/github.com/logrusorgru/news_micro_storage_system?status.svg)](https://godoc.org/github.com/logrusorgru/news_micro_storage_system)
[![WTFPL License](https://img.shields.io/badge/license-wtfpl-blue.svg)](http://www.wtfpl.net/about/)
<!--
[![Build Status](https://travis-ci.org/logrusorgru/news_micro_storage_system.svg)](https://travis-ci.org/logrusorgru/news_micro_storage_system)
[![Coverage Status](https://coveralls.io/repos/logrusorgru/news_micro_storage_system/badge.svg?branch=master)](https://coveralls.io/r/logrusorgru/news_micro_storage_system?branch=master)
[![GoReportCard](https://goreportcard.com/badge/logrusorgru/news_micro_storage_system)](https://goreportcard.com/report/logrusorgru/news_micro_storage_system)
-->

The _news_micro_storage_system_ is microservices to obtain news from
CockroachDB. The news schema is

```sql
id   serial
head varchar(255)
data text
```

# Get

```
go get -u github.com/logrusorgru/news_micro_storage_system
cd $GOPATH/github.com/logrusorgru/news_micro_storage_system
```

Regenerate protbuf messages if you want

```
go generate
```

### Test

Prepare CockroachDB for tests.

```
cockroach start --insecure --listen-addr=localhost
```

Open Cockroach SQL console

```
cockroach sql --insecure
```

And create test database and user.

```sql
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
go test -cover -race ./...
```

To test it with own DB name, DB user name, etc use commandline flags and
test `storage/` and `queryClient/` packages separately. For example


```
cd storage/
go test -cover -race \
    -test-db-addr=localhost            \
    -test-db-port=26257                \
    -test-db-name=test_news_items      \
    -test-db-user=test_news_items      \
    -test-nats-subject=test_news_items
```

and

```
cd queryClient/
go test -cover -race \
    -test-addr=127.0.0.1:3000          \
    -test-timeout=500ms                \
    -test-nats-subject=test_news_items
```

# Start

### CockroachDB

Start CockroachDB server

```
cockroach start --insecure --listen-addr=localhost
```

Craete user and database

```
cockroach sql --insecure
```

```sql
CREATE DATABASE news_items;
CREATE USER news_items;
GRANT ALL ON DATABASE news_items TO news_items;
```

optioanlly, create and fill table

```sql
CREAT TABLE IF NOT EXISTS news_items (
  id   SERIAL PRIMARY KEY,
  head VARCHAR(255),
  data TEXT
);

INSERT INTO news_items (header, data) VALUES
  ('head-1', 'data-1'),
  ('head-2', 'data-2'),
  ('head-3', 'data-3')
RETURNING id;
```

Start NATS server

```
nats-server
```

Start stoarge service
```
go run github.com/logrusorgru/news_micro_storage_system/cmd/storage
```

Start query_client REST service

```
go run github.com/logrusorgru/news_micro_storage_system/cmd/query_client
```

Use with `-h` to see command line flags.


# Query


For example

```
curl -v http://127.0.0.1:3000/news/1
```

# Licensing

Copyright © 2019 Konstantin Ivanov <kostyarin.ivanov@gmail.com>  
This work is free. You can redistribute it and/or modify it under the
terms of the Do What The Fuck You Want To Public License, Version 2,
as published by Sam Hocevar. See the LICENSE file for more details.
