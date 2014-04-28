package main

import (
  "flag"
  "fmt"
  "github.com/gorilla/websocket"
  "log"
  "net/http"
  "encoding/json"
  "labix.org/v2/mgo"
  "labix.org/v2/mgo/bson"
)

var connections map[*websocket.Conn]bool

var (
  session *mgo.Session
  collection *mgo.Collection
)

type Message struct {
  Id bson.ObjectId `bson:"_id" json:"id"`
  Content string `json:"content"`
}

type MessageJSON struct {
  Message Message `json:"message"`
}

type MessagesJSON struct {
  Messages []Message `json:"messages"`
}

func CreateMessageHandler(content string) {
  message := new(Message)

  obj_id := bson.NewObjectId()
  message.Id = obj_id
  message.Content = content

  err := collection.Insert(&message)
  if err != nil {
    panic(err)
  } else {
    log.Printf("Inserted new message %s with content %s", message.Id, message.Content)
  }
}

func MessagesHandler(w http.ResponseWriter, r *http.Request) {
  var mymessages []Message

  iter := collection.Find(nil).Iter()
  result := Message{}
  for iter.Next(&result) {
    mymessages = append(mymessages, result)
  }

  w.Header().Set("Content-Type", "application/json")
  j, err := json.Marshal(MessagesJSON{Messages: mymessages})
  if err != nil { panic(err) }
  w.Write(j)
}

func sendAll(msg []byte) {
  for conn := range connections {
    if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
      delete(connections, conn)
      conn.Close()
    }
  }
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
  // Taken from gorilla's website
  conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
  if _, ok := err.(websocket.HandshakeError); ok {
    http.Error(w, "Not a websocket handshake", 400)
    return
  } else if err != nil {
    log.Println(err)
    return
  }
  log.Println("Succesfully upgraded connection")
  connections[conn] = true

  for {
    // Blocks until a message is read
    _, msg, err := conn.ReadMessage()
    if err != nil {
      delete(connections, conn)
      conn.Close()
      return
    }
    log.Println(string(msg))
    CreateMessageHandler(string(msg))
    sendAll(msg)
  }
}

func main() {
  port := flag.Int("port", 80, "port to serve on")
  dir := flag.String("directory", "www/", "directory of web files")
  flag.Parse()

  log.Println("Starting mongo db session")

  session, err := mgo.Dial("localhost")
  if err != nil { panic(err) }
  defer session.Close()
  session.SetMode(mgo.Monotonic, true)
  collection = session.DB("Messages").C("messages")

  connections = make(map[*websocket.Conn]bool)

  // handle all requests by serving a file of the same name
  fs := http.Dir(*dir)
  fileHandler := http.FileServer(fs)
  http.Handle("/", fileHandler)
  http.HandleFunc("/ws", wsHandler)
  http.HandleFunc("/messages", MessagesHandler)

  log.Printf("Running on port     %d\n", *port)

  addr := fmt.Sprintf("127.0.0.1:%d", *port)
  // this call blocks -- the progam runs here forever
  err = http.ListenAndServe(addr, nil)
  fmt.Println(err.Error())
}
