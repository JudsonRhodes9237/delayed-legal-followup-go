package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	c, err := NewInfraiClient()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := c.CreateQueue("legal-followup-dlq"); err != nil {
		log.Fatal(err)
	}
	if _, err := c.CreateQueue("legal-followups", "legal-followup-dlq"); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/jobs/follow-up", followUpHandler(c))
	fmt.Println("listening on :8080; POST matters to the scheduler from your application")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
