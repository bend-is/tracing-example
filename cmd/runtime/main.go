package main

import (
	"context"
	"os"
	"runtime/trace"
	"time"
)

const orderID = "100"

func main() {
	// go run main.go 2> trace.out
	trace.Start(os.Stderr)
	defer trace.Stop()

	ctx := context.Background()
	ctx, task := trace.NewTask(ctx, "makeCappuccino")
	trace.Log(ctx, "orderID", orderID)

	milk := make(chan bool)
	espresso := make(chan bool)

	go func() {
		trace.WithRegion(ctx, "steamMilk", steamMilk)
		milk <- true
	}()
	go func() {
		trace.WithRegion(ctx, "extractCoffee", extractCoffee)
		espresso <- true
	}()
	defer task.End() // When assemble is done, the order is complete.
	<-espresso
	<-milk
	trace.WithRegion(ctx, "mixMilkCoffee", mixMilkCoffee)
}

func steamMilk() {
	time.Sleep(time.Second * 3)
}

func extractCoffee() {
	time.Sleep(time.Second * 3)
}

func mixMilkCoffee() {
	time.Sleep(time.Second * 3)
}
