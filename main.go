package main

import (
	"fmt"
	"log"

	proto "github.com/manik_taneja/object_replicator/proto"
	"github.com/micro/go-micro"
	"golang.org/x/net/context"
)

/*
Replicator service for an object store tenant. Replicates a minio instance
*/

type Replicator struct {
	bucketReplicators map[string]*BucketReplicator
}

func (r *Replicator) Hello(ctx context.Context, req *proto.HelloRequest, rsp *proto.HelloResponse) error {
	rsp.Greeting = "Hello " + req.Name
	return nil
}

func (r *Replicator) StartReplication(ctx context.Context,
	req *proto.StartReplicationRequest, rsp *proto.StartReplicationResponse) error {

	// TODO generate NATS host from SrcEndpoint

	br, err := newBucketReplicator(req.SrcEndpoint,
		req.SrcAccessKey,
		req.SrcSecretKey,
		req.DstEndpoint,
		req.DstAccessKey,
		req.DstSecretKey,
		"localhost:4222",
		req.Bucket,
		"us-east-1",
	)

	if err != nil {
		log.Printf("Failed to start bucket replicator %v", err)
		return err
	}

	if _, ok := r.bucketReplicators[br.id]; ok == true {
		log.Printf("Replication connection for this bucket already exists")
		return fmt.Errorf("Replication connection exists")
	}

	r.bucketReplicators[br.id] = br
	rsp.ReplicationId = br.id
	return nil
}

func (r *Replicator) StopReplication(ctx context.Context,
	req *proto.ReplicationRequest, rsp *proto.ReplicationResponse) error {
	log.Printf("Pause Replication.")
	br, ok := r.bucketReplicators[req.ReplicationId]
	if ok == false {
		return fmt.Errorf("Replication Id not found")
	}
	close(br.doneChan)
	delete(r.bucketReplicators, req.ReplicationId)
	return nil
}

func (r *Replicator) ResumeReplication(ctx context.Context,
	req *proto.ReplicationRequest, rsp *proto.ReplicationResponse) error {
	log.Printf("Resume Replication.")
	return fmt.Errorf("Not supported")
}

func (r *Replicator) ListEndpoints(ctx context.Context,
	req *proto.ReplicationRequest, rsp *proto.ListEndpointsResponse) error {
	log.Printf("List Endpoints")
	endpointsList := make([]*proto.ReplicationEndpoint, 0)
	for _, br := range r.bucketReplicators {
		re := &proto.ReplicationEndpoint{
			SrcEndpoint:   br.srcEndpoint,
			DstEndpoint:   br.dstEndpoint,
			State:         br.state,
			Bucket:        br.bucket,
			ReplicationId: br.id,
		}
		endpointsList = append(endpointsList, re)
	}
	rsp.ReplicationEndpoints = endpointsList
	return nil
}

func main() {
	// Create a new service. Optionally include some options here.
	service := micro.NewService(
		micro.Name("replicator"),
		micro.Version("latest"),
		micro.Metadata(map[string]string{
			"type": "helloworld",
		}),
	)

	// Init will parse the command line flags. Any flags set will
	// override the above settings. Options defined here will
	// override anything set on the command line.
	service.Init()

	replicator := &Replicator{bucketReplicators: make(map[string]*BucketReplicator)}
	// Register handler
	proto.RegisterReplicatorHandler(service.Server(), replicator)

	// Run the server
	if err := service.Run(); err != nil {
		fmt.Println(err)
	}
}
