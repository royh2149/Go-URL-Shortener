# Go-URL-Shortener
A bit.ly style URL shortener written in Golang

Code snippets taken from:

https://codegangsta.gitbooks.io/building-web-apps-with-go/content/rendering/html/index.html

https://www.golangprograms.com/example-to-handle-get-and-post-request-in-golang.html

https://www.mongodb.com/blog/post/quick-start-golang--mongodb--how-to-read-documents

https://golangcode.com/handle-ctrl-c-exit-in-terminal/

If you have found a code snippet you have written, don't hesitate to reach out.

To setup the MongoDB server I used docker on Linux:

docker run -d -p 27017:27017 -v ~/{host machine directory}:/data/db --name {mongo docker name} mongo

docker exec -it {mongo docker name} bash

mongo

