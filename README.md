# Consul registrator for Mesos

This software will automatically register your Mesos master and its frameworks into Consul.

This does not register running tasks, either directly on Mesos or through Marathon. For this, see [this repo](https://github.com/apognu/marathon-consul).

## Build

```
$ go get github.com/apognu/mesos-consul-registrator
$ $GOPATH/bin/mesos-consul-registrator
```

You can also pull ```apognu/mesos-consul-registrator``` on Docker.

## Usage

```
$ ./mesos-consul-registrator -h
Usage of /home/apognu/Programming/go/bin/mesos-consul-registrator:
  -duration duration
        interval at which to check for topology change (default 10s)
  -frameworks string
        comma-separated list of Mesos frameworks to discover (eg. 'marathon,chronos') (default "marathon,chronos")
  -healthcheck string
        listen address of healthcheck HTTP server (default ":8080")
  -nodes string
        comma-separated list of Mesos nodes (default "127.0.0.1:5050")
```

You should at least give this program one Mesos node to connect to. It assumes a local agent is running on each an every machine you got.

## Output

Il will register the following entries in Consul's catalog (for example with ```-frameworks=marathon```):

| DNS entry                     |
|-------------------------------|
| master.mesos.service.consul   |
| marathon.mesos.service.consul |

## TODO

 * Pull Mesos nodes from ZooKeeper