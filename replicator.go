package main

import (
	// "io"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/minio/minio-go"
	nats "github.com/nats-io/go-nats"
)

func printMinion() {
	log.Print(`Starting MinioNATS

	────────────▀▄───█───▄▀
	───────────▄▄▄█▄▄█▄▄█▄▄▄
	────────▄▀▀═════════════▀▀▄
	───────█═══════════════════█
	──────█═════════════════════█
	─────█═══▄▄▄▄▄▄▄═══▄▄▄▄▄▄▄═══█
	────█═══█████████═█████████═══█
	────█══██▀────▀█████▀────▀██══█
	───██████──█▀█──███──█▀█──██████
	───██████──▀▀▀──███──▀▀▀──██████
	────█══▀█▄────▄██─██▄────▄█▀══█
	────█════▀█████▀───▀█████▀════█
	────█═════════════════════════█
	────█═════════════════════════█
	────█═══════█▀█▀█▀█▀█▀█═══════█
	────█═══════▀▄───────▄▀═══════█
	───▐▓▓▌═══════▀▄█▄█▄▀═══════▐▓▓▌
	───▐▐▓▓▌▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▐▓▓▌▌
	───█══▐▓▄▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▄▓▌══█
	──█══▌═▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▌═▐══█
	──█══█═▐▓▓▓▓▓▓▄▄▄▄▄▄▄▓▓▓▓▓▓▌═█══█
	──█══█═▐▓▓▓▓▓▓▐██▀██▌▓▓▓▓▓▓▌═█══█
	──█══█═▐▓▓▓▓▓▓▓▀▀▀▀▀▓▓▓▓▓▓▓▌═█══█
	──█══█▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓█══█
	─▄█══█▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▌█══█▄
	─█████▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▌─█████
	─██████▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▌─██████
	──▀█▀█──▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▌───█▀█▀
	─────────▐▓▓▓▓▓▓▌▐▓▓▓▓▓▓▌
	──────────▐▓▓▓▓▌──▐▓▓▓▓▌
	─────────▄████▀────▀████▄
	─────────▀▀▀▀────────▀▀▀▀
	credits:
	http://textart4u.blogspot.com/2013/08/minions-emoticons-text-art-for-facebook.html
	https://gist.githubusercontent.com/belbomemo/b5e7dad10fa567a5fe8a/raw/4ed0c8a82a8d1b836e2de16a597afca714a36606/gistfile1.txt
	`)
}

type BucketReplicator struct {
	id               string
	srcEndpoint      string
	dstEndpoint      string
	seplicationState string
	bucket           string
	s3SrcClient      *s3.S3
	s3RemoteClient   *s3.S3
	session          *session.Session
	natsUrl          string
	doneChan         chan bool
	state            string
}

func printBuckets(s3client minio.Client) {
	buckets, err := s3client.ListBuckets()
	if err != nil {
		log.Printf("error listing buckets: %v\n", err)
	}
	for _, bucket := range buckets {
		log.Printf("found bucket: %v\n", bucket.Name)
	}
}

func upsertBucket(s3Client *s3.S3, region string, bucket string) error {

	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		/*		ACL:    aws.String("bucket-owner-full-control"),
				CreateBucketConfiguration: &s3.CreateBucketConfiguration{
					LocationConstraint: aws.String(region),
				},
		*/
	}

	result, err := s3Client.CreateBucket(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				log.Println(s3.ErrCodeBucketAlreadyExists, aerr.Error())
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				log.Println(s3.ErrCodeBucketAlreadyOwnedByYou, aerr.Error())
			default:
				log.Println(aerr.Error())
				return err
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			log.Println(err.Error())
			return err
		}
	}

	log.Printf("Bucket Created %v", result)
	return nil
}

func addNotification(s3Client minio.Client, region string, bucket string) {
	// on bucket notification
	queueArn := minio.NewArn("minio", "sqs", region, "1", "nats")
	queueConfig := minio.NewNotificationConfig(queueArn)
	queueConfig.AddEvents(minio.ObjectCreatedAll)
	queueConfig.AddEvents(minio.ObjectRemovedAll)

	bucketNotification := minio.BucketNotification{}
	bucketNotification.AddQueue(queueConfig)
	err := s3Client.SetBucketNotification(bucket, bucketNotification)

	if err != nil {
		log.Printf("Unable to set bucket notification %v\n", err)
	} else {
		log.Printf("added bucket notification: %v", queueArn)
	}
}

func getClient(endpoint, access, secret string, encrypt bool) *minio.Client {

	client, err := minio.New(endpoint, access, secret, encrypt)
	if err != nil {
		log.Printf("unable to connect to %s: %v\n", endpoint, err)
		return nil
	}

	log.Printf("connected to: %s\n", endpoint)
	return client
}

func getS3Client(endpoint, accessKey, secretKey string) (*s3.S3, *session.Session) {
	// Configure to use Minio Server
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Region:           aws.String("us-east-1"),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
	}

	if endpoint != "" {
		s3Config.Endpoint = aws.String(endpoint)
	}

	newSession := session.New(s3Config)

	s3Client := s3.New(newSession)
	return s3Client, newSession
}

func newBucketReplicator(srcHost, srcAccess, srcSecret,
	remoteHost, remoteAccess, remoteSecret, natsHost, bucket, region string) (*BucketReplicator, error) {

	natsURL := fmt.Sprintf("nats://%s", natsHost)
	flag.Parse()

	// assumes:
	// 	1. Running minio server.
	//	2. Running natsd server.
	// open connection to remote s3 bucket.
	if srcHost == "" || bucket == "" {
		flag.Usage()
		return nil, fmt.Errorf("must specify both src endpoint and bucket ")
	}

	// should be able to get started
	printMinion()

	log.Printf("Starting sync between src %v access %v secret %v"+
		" remote %v access %v secret %v", srcHost, srcAccess, srcSecret,
		remoteHost, remoteAccess, remoteSecret)

	// create remote client
	s3RemoteClient, _ := getS3Client(remoteHost, remoteAccess, remoteSecret)

	// create src client
	s3SrcClient, sess := getS3Client(srcHost, srcAccess, srcSecret)

	// create the bucket in both locations, if it doesn't exist
	err := upsertBucket(s3SrcClient, region, bucket)
	if err != nil {
		return nil, fmt.Errorf("Unable to create bucket. Error %v", err)
	}
	err = upsertBucket(s3RemoteClient, region, bucket)
	if err != nil {
		return nil, fmt.Errorf("Unable to create bucket. Error %v", err)
	}

	// src client for notificatiion access
	minioSrc := getClient(srcHost, srcAccess, srcSecret, false)
	if minioSrc == nil {
		return nil, fmt.Errorf("Unable to connect to source for notification access")
	}

	// add the notification interest
	addNotification(*minioSrc, region, bucket)

	br := &BucketReplicator{
		id:             bucket + "." + srcHost,
		srcEndpoint:    srcHost,
		dstEndpoint:    remoteHost,
		bucket:         bucket,
		natsUrl:        natsURL,
		s3SrcClient:    s3SrcClient,
		s3RemoteClient: s3RemoteClient,
		session:        sess,
		doneChan:       make(chan bool),
	}

	go br.replicateBucket()
	return br, nil
}

func (br *BucketReplicator) replicateBucket() {

	natsConnection, err := nats.Connect(br.natsUrl)
	if err != nil {
		log.Printf("Unable to connect to NATS URL %v", err)
		br.state = "error"
	}
	log.Printf("Connected to NATS: %s\n", br.natsUrl)

	defer natsConnection.Close()
	downloader := s3manager.NewDownloader(br.session)

	// Subscribe to subject
	log.Print("Subscribing to subject 'bucketevents'\n")
	br.state = "active"
	natsConnection.Subscribe("bucketevents", func(msg *nats.Msg) {

		// Handle the message
		notification := minio.NotificationInfo{}

		err := json.Unmarshal(msg.Data, &notification)
		if err != nil {
			fmt.Printf("err: %v\n", err)
		}

		for _, record := range notification.Records {
			key, err := url.QueryUnescape(record.S3.Object.Key)
			if err != nil {
				log.Printf("unable to escape key name (%s): %v", record.S3.Object.Key,
					err)
			}
			switch record.EventName {
			case "s3:ObjectCreated:Put":
				log.Printf("syncing object: %s/%s\n", record.S3.Bucket.Name,
					key)

				srcFile := fmt.Sprintf("%s%s:%s", "/tmp/", record.S3.Bucket.Name, key)
				bucket := record.S3.Bucket.Name

				file, err := os.Create(srcFile)
				if err != nil {
					fmt.Println("Failed to create file", err)
					return
				}

				numBytes, err := downloader.Download(file,
					&s3.GetObjectInput{
						Bucket: aws.String(record.S3.Bucket.Name),
						Key:    aws.String(key),
					})

				if err != nil {
					log.Printf("Unable to download key %q, %v", key, err)
				}
				for i := 0; i < 5; i++ {
					_, err = br.s3RemoteClient.PutObject(&s3.PutObjectInput{
						Body:          file,
						Bucket:        aws.String(bucket),
						Key:           aws.String(key),
						ContentLength: aws.Int64(numBytes),
					})

					if err != nil {
						log.Printf("Unable to write to write to remote . Error %v", err)
						//retry
					} else {
						break
					}
				}

				file.Close()
				err = os.Remove(srcFile)
				if err != nil {
					log.Printf("unable to delete tmp file %s, %v\n", srcFile, err)
				}

			case "s3:ObjectRemoved:Delete":

				_, err := br.s3RemoteClient.DeleteObject(&s3.DeleteObjectInput{Bucket: aws.String(record.S3.Bucket.Name), Key: aws.String(key)})

				if err != nil {
					log.Printf("error deleting object: %v\n", err)
				} else {
					log.Printf("deleted object: %s\n", key)
				}
			}
		}

	})

	<-br.doneChan
}
