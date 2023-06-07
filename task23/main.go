package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	_ "github.com/lib/pq"
)

type Action struct {
	Action     string `json:"action"`
	InstanceId string `json:"instanceId"`
}

var db *sql.DB

func main() {
	var err error

	// Define PostgreSQL configuration
	username := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DB")
	host := os.Getenv("POSTGRES_HOST")

	// connect to the PostgreSQL db
	connStr := fmt.Sprintf("user=%s dbname=%s password=%s host=%s port=%d sslmode=disable", username, dbname, password, host, 5432)
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		fmt.Println("Could not connect to the database", err)
		return
	}

	// You can also check if the connection was successful by pinging the DB
	err = db.Ping()
	if err != nil {
		fmt.Println("Failed to ping the database", err)
		return
	}

	fmt.Println("Successfully connected to the database!")
	// rest of your code...

	// Serve static files
	http.Handle("/", http.FileServer(http.Dir("./task23")))

	// Handle API requests
	http.HandleFunc("/api", handleAPIRequest)

	// Start the server
	fmt.Println("Server listening on port 80...")
	http.ListenAndServe(":80", nil)
}

func handleAPIRequest(w http.ResponseWriter, r *http.Request) {
	var a Action

	if r.Method == "GET" {
		// Retrieve name from query parameter
		name := r.URL.Query().Get("name")

		// Send response
		response := fmt.Sprintf("Hello, %s!", name)
		fmt.Fprint(w, response)

	} else if r.Method == "POST" {
		err := json.NewDecoder(r.Body).Decode(&a)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		sess := session.Must(session.NewSession(&aws.Config{
			Region: aws.String("ap-northeast-3"),
		}))

		svc := ec2.New(sess)

		if a.Action == "create" {
			runResult, err := svc.RunInstances(&ec2.RunInstancesInput{
				ImageId:      aws.String("ami-0997b4797ae01c920"),
				InstanceType: aws.String("t2.micro"),
				MinCount:     aws.Int64(1),
				MaxCount:     aws.Int64(1),
				KeyName:      aws.String("my-key"),
				SecurityGroupIds: []*string{
					aws.String("sg-0807f948d4a9a56b5"),
				},
				TagSpecifications: []*ec2.TagSpecification{
					{
						ResourceType: aws.String("instance"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String("Name"),
								Value: aws.String("EC2"),
							},
						},
					},
				},
			})

			if err != nil {
				fmt.Println("Could not create instance", err)
				http.Error(w, "Could not create instance: "+err.Error(), http.StatusInternalServerError)
				return
			}

			instanceID := *runResult.Instances[0].InstanceId
			fmt.Println("Created instance", instanceID)

			// save instance id into db
			_, err = db.Exec("INSERT INTO instances(instance_id) VALUES($1)", instanceID)
			if err != nil {
				http.Error(w, "Could not insert instance ID into database: "+err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("EC2 instance created successfully!"))
		} else if a.Action == "terminate" {
			delResult, err := svc.TerminateInstances(&ec2.TerminateInstancesInput{
				InstanceIds: []*string{
					aws.String(a.InstanceId),
				},
			})

			if err != nil {
				fmt.Println("Could not terminate instance", err)
				http.Error(w, "Could not terminate instance: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// delete instance id from db
			_, err = db.Exec("DELETE FROM instances WHERE instance_id=$1", a.InstanceId)
			if err != nil {
				http.Error(w, "Could not delete instance ID from database: "+err.Error(), http.StatusInternalServerError)
				return
			}

			fmt.Println("Terminated instance", *delResult.TerminatingInstances[0].InstanceId)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("EC2 instance terminated successfully!"))
		} else {
			fmt.Println("Invalid action: " + a.Action)                        // Print error to console
			http.Error(w, "Invalid action: "+a.Action, http.StatusBadRequest) // Send detailed error message to client
		}
	}
}
