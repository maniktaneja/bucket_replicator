package main

import (
	"fmt"
	"log"
	"os"

	proto "github.com/manik_taneja/object_replicator/proto"
	"github.com/micro/cli"
	"github.com/micro/go-micro"
	"golang.org/x/net/context"
)

func startReplication(r proto.ReplicatorClient, srcEndpoint, srcAccess, srcSecret,
	dstEndpoint, dstAccess, dstSecret, bucket string) error {

	log.Printf(" Destination Secret %v", dstSecret)
	startrsp, err := r.StartReplication(context.TODO(),
		&proto.StartReplicationRequest{
			SrcEndpoint:  srcEndpoint,
			SrcAccessKey: srcAccess,
			SrcSecretKey: srcSecret,
			DstEndpoint:  dstEndpoint,
			DstAccessKey: dstAccess,
			DstSecretKey: dstSecret,
			Bucket:       bucket})
	if err != nil {
		log.Printf("Unable to start Replication. Error %v", err)
		return err
	} else {
		log.Printf("Replication started with Replication Id: %s", startrsp.ReplicationId)
	}

	return nil
}

func stopReplication(r proto.ReplicatorClient, replID string) error {

	_, err := r.StopReplication(context.TODO(),
		&proto.ReplicationRequest{ReplicationId: replID})
	if err != nil {
		log.Printf("Pause Replication Failed. Error %v", err)
	}

	return err
}

func resumeReplication(r proto.ReplicatorClient, replID string) error {

	_, err := r.ResumeReplication(context.TODO(),
		&proto.ReplicationRequest{ReplicationId: replID})
	if err != nil {
		log.Printf("Resume Replication Failed. Error %v", err)
	}

	return err
}

func listEndpoints(r proto.ReplicatorClient) error {

	listResp, err := r.ListEndpoints(context.TODO(),
		&proto.ReplicationRequest{})
	if err != nil {
		log.Printf("Resume Replication Failed. Error %v", err)
	} else {

		for _, e := range listResp.ReplicationEndpoints {
			log.Printf("Endpoint : %v\n", e)
		}
	}

	return err
}

// Setup and the client
func runClient(service micro.Service) {
	// Create new replicator client
	replicator := proto.NewReplicatorClient("replicator", service.Client())

	// Call the replicator
	rsp, err := replicator.Hello(context.TODO(), &proto.HelloRequest{Name: "John"})
	if err != nil {
		fmt.Println(err)
		return
	}

	// Print response
	fmt.Println(rsp.Greeting)

	startrsp, err := replicator.StartReplication(context.TODO(),
		&proto.StartReplicationRequest{
			SrcEndpoint: "minio endpoint",
			DstEndpoint: "S3 compatible endpoint"})
	if err != nil {
		fmt.Println(err)
		return
	}

	// Print response
	fmt.Println(startrsp)

}

func main() {
	// Create a new service. Optionally include some options here.
	service := micro.NewService(
		micro.Name("replicator"),
		micro.Version("latest"),
		micro.Metadata(map[string]string{
			"type": "helloworld",
		}),

		// Setup some flags. Specify --run_client to run the client

		// Add runtime flags
		// We could do this below too
		micro.Flags(
			cli.StringFlag{
				Name:  "command",
				Usage: "Action to perform: start | stop | resume | list",
			},
			cli.StringFlag{
				Name:  "src_endpoint",
				Usage: "Source S3 compatible endpoint URL",
			},
			cli.StringFlag{
				Name:  "dst_endpoint",
				Usage: "Destination S3 compatible endpoint URL",
			},
			cli.StringFlag{
				Name:  "src_access",
				Usage: "Source endpoint access key",
			},
			cli.StringFlag{
				Name:  "dst_access",
				Usage: "Destination endpoint access key",
			},
			cli.StringFlag{
				Name:  "src_secret",
				Usage: "Source endpoint secret key",
			},
			cli.StringFlag{
				Name:  "dst_secret",
				Usage: "Destination endpoint secret key",
			},
			cli.StringFlag{
				Name:  "bucket",
				Usage: "Bucket to be replicated",
			},
			cli.StringFlag{
				Name:  "replID",
				Usage: "Replication ID that identifies a pair of endpoints",
			},
		),
	)

	// Init will parse the command line flags. Any flags set will
	// override the above settings. Options defined here will
	// override anything set on the command line.
	service.Init(
		// Add runtime action
		// We could actually do this above
		micro.Action(func(c *cli.Context) {

			replicator := proto.NewReplicatorClient("replicator", service.Client())

			switch c.String("command") {
			case "start":
				startReplication(replicator, c.String("src_endpoint"), c.String("src_access"),
					c.String("src_secret"), c.String("dst_endpoint"), c.String("dst_access"),
					c.String("dst_secret"), c.String("bucket"))
			case "stop":
				stopReplication(replicator, c.String("replID"))
			case "resume":
				resumeReplication(replicator, c.String("replID"))
			case "list":
				listEndpoints(replicator)
			default:
				log.Printf(" No command specified")
			}
			os.Exit(0)
		}),
	)

}
