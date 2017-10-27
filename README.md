# Bucket Replicator Sevice
Use Minio to notify of changes through NATS and sync changes between two minio servers or between minio
and s3.
Adapted from : https://github.com/nats-io/demo-minio-nats

# Overview
Minio makes it easy to manage an object store with an S3 interface across multiple different platforms, 
from your local desktop to other clouds beyond AWS.

This replicator service allows you to asyncronously replicate buckets across two cloud storage deployments, 
the source cloud however must be a minio. 

# Assumptions
Run this service on the same host as the source object store
NATS running on localhost:4222

#TODO
Build fault-tolerance and failure handling etc.

![Diagram](/readme_img/diag.png?raw=true "Diagram")

# Tutorial

1. Install and run gnatsd
    ```
    go get github.com/nats-io/gnatsd; gnatsd -D -V
    ```
1. Install minio
    ```
    go get github.com/minio/minio
    ```
1. Configure minio for local NATS event subscription
    
    edit `~/.minio/config.json`
    
    set `"nats"."1"."enable": true`
    
    ``` 
    ...
    "nats": {
        "1": {
            "enable": true,
            "address": "0.0.0.0:4222",
            "subject": "bucketevents",
    ...
    ```
1. Run minio
    
    Set ~/minio-tmp/ to any directory you want to store your objects in.
    
    ```
    minio server ~/minio-tmp/
    ```
1. Install Consul : https://www.consul.io/intro/getting-started/install.html

	This is required for service discovery
	```
	$ consul agent -dev
	```
1. Build both the server and client code and launch the server
	```
	cd proto (not required unless you make any changes to the proto file)
	protoc -I$GOPATH/src --go_out=plugins=micro:$GOPATH/src $GOPATH/src/github.com/manik_taneja/bucket_replicator/proto/replicator.proto
	go build
	cd client
	go build
	cd ../
	./bucket-replicator
	```
1. Use the client program to enable the replication between two object store instances
    ```
	./client --command=start --src_endpoint=localhost:9000 --src_access=source_access_key --src_secret=source_secret_key --dst_access=dest_access_key --dst_secret=dest_secret_key --bucket=ntnx-replicator-test-bucket
    ```

	If the bucket doesn't exist it will be created. If the replication target is S3 then no value needs to be
	specified for the dst_endpoint.
    
1. Open Browsers to your test bucket [Minio Browser](http://localhost:9000/ and 
an [S3 Browser](https://s3.console.aws.amazon.com/s3/buckets/ntnx-replicator-test-bucket/)

1. Upload a File to your Minio Browser. Watch it automatically get added to your S3 browser

1. Delete a File from your Minio Browser. Watch it automatically get removed from your S3 Browser

# Usage
```
	Supported commands
	-start : starts the replication, returns a replication Id
	-stop : stop the replication for a given replication Id
	-list : list all the configured endpoints
```

# Additional Reading
[Publish Minio Events via NATS](https://blog.minio.io/part-4-5-publish-minio-events-via-nats-79114ea5cd29#.s2sifywij)

[NATS Blog](http://nats.io/blog/)
