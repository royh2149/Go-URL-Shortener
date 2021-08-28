package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv" // string manipulation
	"syscall"

	// to generate short URLs
	"math/rand"
	"strings"

	// for the http service
	"html/template"
	"net/http"
	"path"

	// for database communication
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var CHARSET string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890" // charset for shortened URL
var LENGTH int = 6                                                                    // the length of the shortened URL (after domain)
var PORT int = 22222                                                                  // website port
var IP string = "127.0.0.1"                                                           // domain IP

var NO_SUCH_DOCUMENT_ERROR string = "dnf" // document (database record) not found error

var DB_NAME string = "project"            // the name of the urls database
var TABLE_NAME string = "addresses_table" // collection (MongoDB table) name, in which the pairs are stored

// introducing a new type, that holds a URL and its shortened version. Specify matching database "column" names
type ShortPair struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Url       string             `bson:"url" json:"url"`
	Shortened string             `bson:"shortened" json:"shortened"`
}

var collection *mongo.Collection // the url pairs collection (table)

func main() {
	// Set client options
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")

	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), clientOptions)

	if err != nil { // ensure no error occurred
		log.Fatal(err)
	}

	SetupCloseHandler(client) // cleanly exit the program if Ctrl+c is pressed (for example)

	// Check the connection
	err = client.Ping(context.TODO(), nil)

	if err != nil { // make sure the connection to the database is solid
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB!")

	collection = client.Database(DB_NAME).Collection(TABLE_NAME) // get a handle to the urls collection

	http.HandleFunc("/", HandleWebSite)              // specify which function should handle website requests
	http.ListenAndServe(":"+strconv.Itoa(PORT), nil) // setup the http server, with IP and port
}

func HandleWebSite(writer http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" { // if the requested page is not the home page, redirect the user to the appropriate URL (if exists)
		originalUrl := GetOriginalUrl(req.URL.Path[1:]) // query the DB for the original URL of the received shortened version

		if originalUrl == NO_SUCH_DOCUMENT_ERROR { // if no shortened URL exists, serve the error file
			http.ServeFile(writer, req, "./views/error.html")
			return
		}

		// redirect the user to the corresponding URL
		if strings.HasPrefix(originalUrl, "http://") || strings.HasPrefix(originalUrl, "https://") { // check if the URL begins with http:// or https://
			http.Redirect(writer, req, originalUrl, http.StatusSeeOther)
		} else { // add the http prefix if necessary
			http.Redirect(writer, req, "http://"+originalUrl, http.StatusSeeOther)
		}

		return
	}

	switch req.Method { // determine the HTTP method used
	case "GET": // in a regular GET method, serve the home page file
		http.ServeFile(writer, req, "./views/index.html")
	case "POST": // in case of a post request, generate a shortened version for the received URL and add it to the DB
		// parse the raw query and update req.PostForm and req.Form
		if err := req.ParseForm(); err != nil {
			fmt.Fprintf(writer, "ParseForm() err: %v", err)
			return
		}

		sourceUrl := req.FormValue("source_url") // get the entered URL

		shortUrl := GenerateShortenedUrl() // generate a shortened version for the URL

		for ShortAlreadyUsed(shortUrl) { // while loop until a non-used shortened URL is found
			shortUrl = GenerateShortenedUrl()
		}

		AddUrlPair(sourceUrl, shortUrl) // add the new URL pair to the DB

		// create a new pair with the appropriate values
		pair := ShortPair{Url: sourceUrl, Shortened: IP + ":" + strconv.Itoa(PORT) + "/" + shortUrl}

		fp := path.Join("views", "pair.html") // prepare the template file
		tmpl, err := template.ParseFiles(fp)  // parse the template file for later processing
		if err != nil {                       // check for an error
			http.Error(writer, err.Error(), http.StatusInternalServerError) // issue an error
			return
		}

		// show the user his shortened URL, issue an error in case of failure
		if err := tmpl.Execute(writer, pair); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
		}

	default: // other HTTP methods are not relevant
		fmt.Fprintf(writer, "Sorry, Only GET/POST methods are supported. ")
	}
}

func GenerateShortenedUrl() string { // returns a random sequence of chars from CHARSET, in the length of LENGTH
	short := "" // the result string

	for i := 0; i < LENGTH; i++ { // 6 chars total
		index := rand.Intn(len(CHARSET)) // get a random index between 0 and the length of CHARSET
		short += string(CHARSET[index])  // add the randomly picked char to the result string
	}

	return short // return the generated shortened URL
}

func ShortAlreadyUsed(shortUrl string) bool { // returns true if the received shortened address is already in use
	var pair bson.M // to store the query result

	// search for the received shortened URL in the database
	err := collection.FindOne(context.TODO(), bson.M{"shortened": shortUrl}).Decode(&pair)

	// check if an error occurred
	if err != nil {
		if err == mongo.ErrNoDocuments { // if no docs were retrieved, it means the shortUrl is free to use. Return false
			return false
		} else { // in case of other errors, log them
			log.Fatal(err)
		}
	}

	return true // if the code arrived here, it means the received shortUrl is already in use
}

func AddUrlPair(sourceUrl string, shortUrl string) { // adds a new document to the URLs collection
	newPair := ShortPair{Url: sourceUrl, Shortened: shortUrl} // create the new pair
	_, err := collection.InsertOne(context.TODO(), newPair)   // insert the created pair to the database
	if err != nil {                                           // ensure no error occurred
		log.Fatal(err)
	}
}

func GetOriginalUrl(shortUrl string) string { // returns the original URL whose shortened version is the provided shortUrl
	// search for the record with the received shortened URL
	var pair ShortPair // to store the query result

	// search for the received shortened URL in the database
	err := collection.FindOne(context.TODO(), bson.M{"shortened": shortUrl}).Decode(&pair)

	// ensure no error occurred
	if err != nil {
		if err == mongo.ErrNoDocuments { // indicate that the received URL doesn't exist
			return NO_SUCH_DOCUMENT_ERROR
		} else {
			log.Fatal(err)
		}
	}

	return pair.Url // return the original URL
}

// SetupCloseHandler creates a 'listener' on a new goroutine which will notify the
// program if it receives an interrupt from the OS. We then handle this by calling
// our clean up procedure and exiting the program.
func SetupCloseHandler(client *mongo.Client) {
	c := make(chan os.Signal)                       // create a listening channel
	signal.Notify(c, os.Interrupt, syscall.SIGTERM) // specify that SIGTERM signal, such as Ctrl+c, will go to the channel

	go func() { // a goroutine that waits for the sigterm, and then cleanly exits the program
		<-c // a blocking action, waits for the SIGTERM
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		client.Disconnect(context.TODO()) // close the connection to the DB
		os.Exit(0)                        // exit the program
	}()
}
