package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/mnmtanish/go-graphiql"
	"github.com/neo4j/neo4j-go-driver/neo4j"
	"github.com/rs/cors"
)

var (
	query string
)

type Person struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func processHeaders(result neo4j.Result) {
	if keys, err := result.Keys(); err == nil {
		for index, key := range keys {
			if index > 0 {
				fmt.Print("\t")
			}
			fmt.Printf("%-10s", key)
		}
		fmt.Print("\n")

		for index := range keys {
			if index > 0 {
				fmt.Print("\t")
			}
			fmt.Print(strings.Repeat("=", 10))
		}
		fmt.Print("\n")
	}
}

// Simple record values printing logic, open to improvements
func processRecord(record neo4j.Record) {
	for index, value := range record.Values() {
		if index > 0 {
			fmt.Print("\t")
		}
		fmt.Printf("%-10v", value)
	}
	fmt.Print("\n")
}

// Transaction function
func executeQuery(tx neo4j.Transaction) (interface{}, error) {
	var (
		counter int
		result  neo4j.Result
		err     error
	)

	// Execute the query on the provided transaction
	if result, err = tx.Run(query, nil); err != nil {
		return nil, err
	}

	// Print headers
	processHeaders(result)

	// Loop through record stream until EOF or error
	for result.Next() {
		processRecord(result.Record())
		counter++
	}
	// Check if we encountered any error during record streaming
	if err = result.Err(); err != nil {
		return nil, err
	}

	// Return counter
	return counter, nil
}

// var People []Person
func getPeople(limit int) []Person {
	// Neo4j
	configForNeo4j40 := func(conf *neo4j.Config) { conf.Encrypted = false }

	driver, err := neo4j.NewDriver("bolt://localhost:7687", neo4j.BasicAuth("neo4j", "test", ""), configForNeo4j40)
	if err != nil {
		log.Println("error connecting to neo4j:", err)
	}
	defer driver.Close()
	cypher := `MATCH (n:Person) RETURN ID(n) as id, n.name as name LIMIT $limit`
	sessionConfig := neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead}
	session, err1 := driver.NewSession(sessionConfig)
	if err1 != nil {
		log.Println("error connecting to neo4j:", err1)
	}
	defer session.Close()

	result, err := session.Run(cypher, map[string]interface{}{"limit": limit})
	if err != nil {
		log.Println("error querying person:", err)
	}

	results := make([]Person, 0)
	for result.Next() {
		results = append(results, Person{
			ID:   result.Record().GetByIndex((0)).(int64),
			Name: result.Record().GetByIndex((1)).(string),
		})
	}
	return results
}

var personType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Person",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
		},
	},
)
var queryType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Query",
	Fields: graphql.Fields{
		"People": &graphql.Field{
			Type:        graphql.NewList(personType),
			Description: "List of people",
			Args: graphql.FieldConfigArgument{
				"limit": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				// People = getPeople()
				return getPeople(p.Args["limit"].(int)), nil
			},
		},
	},
})
var Schema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query: queryType,
})

func main() {
	log.Print("Starting server...")
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})
	h := handler.New(&handler.Config{
		Schema: &Schema,
		Pretty: true,
	})

	// serve HTTP
	serveMux := http.NewServeMux()
	// serveMux.HandleFunc("/neo", neo4jHandler)
	serveMux.Handle("/graphql", c.Handler(h))
	serveMux.HandleFunc("/graphiql", graphiql.ServeGraphiQL)
	log.Print("Server up!")
	log.Fatal(http.ListenAndServe(":8080", serveMux))
}
