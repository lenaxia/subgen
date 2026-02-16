package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/your-org/subgen/orchestrator/pkg/pb"
)

func main() {
	// Connect to worker
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewTranscriptionServiceClient(conn)

	// Test DetectLanguage with file path
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.DetectLanguageRequest{
		AudioSource: &pb.DetectLanguageRequest_FilePath{
			FilePath: "/output/speech_sample.wav",
		},
		SampleLength: 10,
		SampleOffset: 0,
	}

	fmt.Println("Sending DetectLanguage request...")
	resp, err := client.DetectLanguage(ctx, req)
	if err != nil {
		log.Fatalf("DetectLanguage failed: %v", err)
	}

	if resp.Success {
		fmt.Printf("✅ Language Detection Success!\n")
		fmt.Printf("   Language Code: %s\n", resp.LanguageCode)
		fmt.Printf("   Language Name: %s\n", resp.LanguageName)
		fmt.Printf("   Confidence: %.2f\n", resp.Confidence)
	} else {
		fmt.Printf("❌ Language Detection Failed: %s\n", resp.ErrorMessage)
	}
}
