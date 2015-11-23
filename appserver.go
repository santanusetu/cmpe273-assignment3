package main

import (
    "fmt"
    "strings"
    "errors"
    "net/http"
    "io/ioutil"
    "encoding/json"
    "gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
    "sort"
    "github.com/jmoiron/jsonq"
    "bytes"
    "strconv"
    "github.com/julienschmidt/httprouter"
    "github.com/r-medina/go-uber"    
)

type Tdata struct {
    Id                          bson.ObjectId       `json:"id" bson:"_id"`
    Status                      string              `json:"status" bson:"status"`
    Starting_from_location_id   string              `json:"starting_from_location_id" bson:"starting_from_location_id"`
    Best_route_location_ids     []string            `json:"best_route_location_ids" bson:"best_route_location_ids"`
    Total_uber_costs            int                 `json:"total_uber_cost" bson:"total_uber_cost"`
    Total_uber_duration         int                 `json:"total_uber_duration" bson:"total_uber_duration"`
    Total_distance              float64             `json:"total_distance" bson:"total_distance"` 
}

type eta struct {
    Eta                         int                 `json:"eta"`
    RequestID                   string              `json:"request_id"`
    Status                      string              `json:"status"`
    SurgeMultiplier             float64             `json:"surge_multiplier"`
}

type Tstatus struct {
    Id                          bson.ObjectId       `json:"id" bson:"_id"`
    Status                      string              `json:"status" bson:"status"`
    Starting_from_location_id   string              `json:"starting_from_location_id" bson:"starting_from_location_id"`
    Next_destination_location_id string             `json:"next_destination_location_id" bson:"next_destination_location_id"`
    Best_route_location_ids     []string            `json:"best_route_location_ids" bson:"best_route_location_ids"`
    Total_uber_costs            int                 `json:"total_uber_cost" bson:"total_uber_cost"`
    Total_uber_duration         int                 `json:"total_uber_duration" bson:"total_uber_duration"`
    Total_distance              float64             `json:"total_distance" bson:"total_distance"` 
    Uber_wait_time_eta          int                 `json:"uber_wait_time_eta" bson:"uber_wait_time_eta"`
}

type request struct {
    LocationIds                 []string            `json:"location_ids"`
    StartingFromLocationID      string              `json:"starting_from_location_id"`
}


type Udata struct {
    Id                          bson.ObjectId        `json:"id" bson:"_id"`
    Name                        string               `json:"name" bson:"name"`
    Address                     string               `json:"address" bson:"address"`
    City                        string               `json:"city" bson:"city"`
    State                       string               `json:"state" bson:"state"`
    Zip                         string               `json:"zip" bson:"zip"`
    Coordinate struct {
        Lat        float64         `json:"lat" bson:"lat"`
        Lng        float64         `json:"lng" bson:"lng"`
    } `json:"coordinate" bson:"coordinate"`
}


type Data struct{
        id          string
        price       int
        duration    int
        distance    float64
    }


type coordinate struct {
        lat         float64
        lng         float64
    }


var nextid string
var startid string
var Locids []string
type dataSlice []Data


// Function to initiate Mongo Session
func getMgoSession() *mgo.Session {
    session, err := mgo.Dial("mongodb://santanu:santanu@ds045054.mongolab.com:45054/assignment2")
    if err != nil {
        panic(err)
    }
    return session
}


func (d dataSlice) Len() int {
	return len(d)
}

func (d dataSlice) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func (d dataSlice) Less(i, j int) bool {
	return d[i].price < d[j].price 
}

//Function to find least price for the route
func getLeastData(x map[string]Data)(y Data) {
	m := x
	s := make(dataSlice, 0, len(m))
	for _, d := range m {
		s = append(s, d)
	}		
	sort.Sort(s)
	return s[0]
}

//Function to remove
func removeData(s []string, p string)(x []string) {
    var r []string
    for _, str := range s {
        if str != p {
            r = append(r, str)
        }
    }
    return r
}

func CostInFloat(a []float64) (sum float64) {
    for _, v := range a {
        sum += v
    }
    return
}

func CostInInt(a []int) (sum int) {
    for _, v := range a {
        sum += v
    }
    return
}


//Function to go back to home location
func getPriceToHome(x string)(y Data){
    var pricetohome []int
    response, err := http.Get(x)
    if err != nil { return }
    defer response.Body.Close()
    resp := make(map[string]interface{})
    body, _ := ioutil.ReadAll(response.Body)
    err = json.Unmarshal(body, &resp)
    if err != nil { return }
    ptr := resp["prices"].([]interface{})
    jnquery := jsonq.NewQuery(resp)
    for i, _ := range ptr {
        priceshome,_ := jnquery.Int("prices",fmt.Sprintf("%d", i),"low_estimate")
        pricetohome = append(pricetohome, priceshome)
    }
    min := pricetohome[0]
    for j, _ := range pricetohome {
        if(pricetohome[j]<=min && pricetohome[j]!=0){
            min = pricetohome[j]
        }
    }
    durationhome,_:= jnquery.Int("prices","0","duration")
    distancehome,_:= jnquery.Float("prices","0","distance")
    data := Data{
        id:"",
        price : min,
        duration:durationhome,
        distance:distancehome,
    }
    return data
}


//Function to get trip details
func getTripDetails(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
    tripid :=  p.ByName("tripid")
    if !bson.IsObjectIdHex(tripid) {
        rw.WriteHeader(404)
        return
    }

    dataid := bson.ObjectIdHex(tripid)
    responseObj := Tdata{}
    if err := getMgoSession().DB("assignment2").C("locationsdata").FindId(dataid).One(&responseObj); err != nil {
        rw.WriteHeader(404)
        return
    }
    reply, _ := json.Marshal(responseObj)
    rw.Header().Set("Content-Type", "application/json")
    rw.WriteHeader(200)
    fmt.Fprintf(rw, "%s", reply)
}

//Function to find shortest route
func findShortestRoute(rw http.ResponseWriter, req *http.Request, p httprouter.Params){
    decoder := json.NewDecoder(req.Body)
    var t request
    err := decoder.Decode(&t)
    if err != nil {
        panic(err)
    }
    Start := t.StartingFromLocationID
    LocIds := t.LocationIds
    var tripData Tdata
    var coord coordinate
    var tripint []int
    var tripdurationfloat []float64
    var tripduration []int

   for arraylength:=len(LocIds); arraylength>0; arraylength--{
    coord = getDetails(Start)
    start_lat := coord.lat
    start_lng := coord.lng
    x := []coordinate{}
    for i := 0; i < len(LocIds); i++ {
       y := getDetails(LocIds[i])
       x = append(x,y)
   }
   tdata := map[string]Data{}
      for i:=0;i<len(x);i++{
      url := fmt.Sprintf("https://sandbox-api.uber.com/v1/estimates/price?start_latitude=%f&start_longitude=%f&end_latitude=%f&end_longitude=%f&server_token=tyC8DGEaUgBO68yd4rE9RIKF4PyweeZq0uH-bz9-",start_lat,start_lng,x[i].lat,x[i].lng)
      d:= getPrice(url, LocIds[i])
      tdata[LocIds[i]] = d
      }
   da:= getLeastData(tdata)
   tripData.Best_route_location_ids = append(tripData.Best_route_location_ids,da.id)
   tripint = append(tripint,da.price)
   tripdurationfloat = append(tripdurationfloat,da.distance)
   tripduration = append(tripduration,da.duration)
   LocIds= removeData(LocIds,da.id)
   Start=da.id
   }
   if(LocIds==nil){
   coord = getDetails(Start)
    start_lat := coord.lat
    start_lng := coord.lng
    x := coordinate{}
    y := getDetails(t.StartingFromLocationID)
    x.lat=y.lat
    x.lng=y.lng
       tdata := map[string]Data{}
      url := fmt.Sprintf("https://sandbox-api.uber.com/v1/estimates/price?start_latitude=%f&start_longitude=%f&end_latitude=%f&end_longitude=%f&server_token=tyC8DGEaUgBO68yd4rE9RIKF4PyweeZq0uH-bz9-",start_lat,start_lng,x.lat,x.lng)
      d:= getPriceToHome(url)
      tdata[Start] = d
   tripint = append(tripint,d.price)
   tripdurationfloat = append(tripdurationfloat,d.distance)
   tripduration = append(tripduration,d.duration)
   }

    tripData.Id = bson.NewObjectId()
    tripData.Status = "Planning"
    tripData.Starting_from_location_id= t.StartingFromLocationID
    tripData.Best_route_location_ids = tripData.Best_route_location_ids
    tripData.Total_uber_costs = CostInInt(tripint)
    tripData.Total_uber_duration = CostInInt(tripduration)
    tripData.Total_distance = CostInFloat(tripdurationfloat)
    getMgoSession().DB("assignment2").C("locationsdata").Insert(tripData)

    reply, _ := json.Marshal(tripData)
    rw.Header().Set("Content-Type", "application/json")
    rw.WriteHeader(201)
    fmt.Fprintf(rw, "%s", reply)
    }




//Function to get Estimated Time of Arrival
func getEstimatedTimeOfArrival(x float64,y float64,z string)(p int){
    lat := strconv.FormatFloat(x, 'E', -1, 64)
    lng := strconv.FormatFloat(y, 'E', -1, 64)
    url := "https://sandbox-api.uber.com/v1/requests"
    var jsonStr = []byte(`{
"start_latitude":"`+lat+`",
"start_longitude":"`+lng+`",
"product_id":"`+z+`",
}`)
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
    req.Header.Set("Authorization", "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsicHJvZmlsZSIsInJlcXVlc3RfcmVjZWlwdCIsInJlcXVlc3QiLCJoaXN0b3J5X2xpdGUiXSwic3ViIjoiMzVjYzM1NzctMjZjZC00YjVjLWFiYzEtODZiOGVlNWZlZTQ3IiwiaXNzIjoidWJlci11czEiLCJqdGkiOiIxZjZmOGIzZS1hMWJhLTQ1NDgtOTNkYS1jNDYxNWY0YWNhYTgiLCJleHAiOjE0NTA0Mzg5MDQsImlhdCI6MTQ0Nzg0NjkwMywidWFjdCI6IkxQSXB4d3V5eUE1UW9YNzRtWlBRWHVkZ01pUTZUeiIsIm5iZiI6MTQ0Nzg0NjgxMywiYXVkIjoiOFlaRWNXc21zX0d3QU1zTlFtOHhyMF94aWFOazhpa3UifQ.BNrFhtQxxFddp542eyaSxDD4peLzdbUaDs7fDeeRixAjyhGtvcyUmDZuNN4lAuYU9ETqbmsUx6AcRat0Pc9ZczuISPDlDxMS9bzJgFIlHFotaMOflkUDJS-ffaB35VzH-j1y2EXyFFQvYNesX5BOQVuwieLXS7sjef1Efz36UL6_MX36_Lq4p0QmO2HtDgo7YHXFo2z4n4DnaHIgIEMFrm0T9nK4D6Zlf0BySf5CPu5AfuOpNj46MY6ZFh3WlqLJFCdWgX7Wyd5U4rh9zJyrwopcwFfP3C0QddcxR-cuxDQuYaHX-OHDcWsXyf2NSmhEo_tw1caAt_xRfK3xhaTOPw")
     req.Header.Set("Content-Type", "application/json")
    var resp1 eta
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    body, _ := ioutil.ReadAll(resp.Body)
    err = json.Unmarshal(body,&resp1)
    if err != nil {
        panic(err)
    }
    rid:= resp1.Eta
    fmt.Println(resp1)
    return rid
}


//Function to enter tripid details
func requestTrip(rw http.ResponseWriter, req *http.Request, p httprouter.Params){
    client := uber.NewClient("JAp1znEmX039zT-wk87nYABr62QAT6aoRFYpxGLo")
    tripid :=  p.ByName("tripid")
    if !bson.IsObjectIdHex(tripid) {
        rw.WriteHeader(404)
        return
    }
    dataid := bson.ObjectIdHex(tripid)
    responseObj := Tdata{}
    if err := getMgoSession().DB("assignment2").C("locationsdata").FindId(dataid).One(&responseObj); err != nil {
        rw.WriteHeader(404)
        return
    }
    if(nextid==""){
    startid =responseObj.Starting_from_location_id
     Locids =responseObj. Best_route_location_ids
    z := getDetails(responseObj.Starting_from_location_id)
    start_lat := z.lat
    start_lng := z.lng
    products,_ := client.GetProducts(start_lat,start_lng)
    productid := products[0].ProductID
    eta:= getEstimatedTimeOfArrival(start_lat,start_lng,productid)
    nextid = Locids[0]
    reply := Tstatus{
    Id:responseObj.Id,
    Starting_from_location_id :startid, 
    Best_route_location_ids:responseObj. Best_route_location_ids,
    Total_uber_costs:responseObj.Total_uber_costs,
    Total_uber_duration:responseObj.Total_uber_duration,
    Total_distance:responseObj.Total_distance,
    Uber_wait_time_eta: eta,
     Status : "Requesting",
     Next_destination_location_id: nextid,
  }
  getMgoSession().DB("assignment2").C("locationsdata").Update(bson.M{"_id":dataid }, bson.M{"$set": bson.M{ "status": "Requesting"}})
  startid = nextid
  Locids= removeData(Locids,nextid)
  if(Locids!=nil){
  nextid = Locids[0]
  }else{
  nextid = "empty"
  }
    res, _ := json.Marshal(reply)
    rw.Header().Set("Content-Type", "application/json")
    fmt.Fprintf(rw, "%s", res)
    }else if(Locids!=nil){
    if(nextid!="empty"){
    z := getDetails(startid)
    start_lat := z.lat
    start_lng := z.lng
    products,_ := client.GetProducts(start_lat,start_lng)
    productid := products[0].ProductID
    eta:= getEstimatedTimeOfArrival(start_lat,start_lng,productid)
    reply := Tstatus{
    Id:responseObj.Id,
    Starting_from_location_id :startid, 
    Best_route_location_ids:responseObj. Best_route_location_ids,
    Total_uber_costs:responseObj.Total_uber_costs,
    Total_uber_duration:responseObj.Total_uber_duration,
    Total_distance:responseObj.Total_distance,
    Uber_wait_time_eta: eta,
        Status : "Requesting",
     Next_destination_location_id: nextid,
     }
     getMgoSession().DB("assignment2").C("locationsdata").Update(bson.M{"_id":dataid }, bson.M{"$set": bson.M{ "status": "Requesting"}})
     startid = nextid
      Locids= removeData(Locids,nextid)
      if(Locids!=nil){
      nextid = Locids[0]
      }else{
      nextid = "empty"
      }
      res, _ := json.Marshal(reply)
        rw.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(rw, "%s", res)
        }
    }else if(nextid=="empty"){
        z := getDetails(startid)
        start_lat := z.lat
        start_lng := z.lng
        products,_ := client.GetProducts(start_lat,start_lng)
        productid := products[0].ProductID
        eta:= getEstimatedTimeOfArrival(start_lat,start_lng,productid)
        reply := Tstatus{
        Id:responseObj.Id,
        Starting_from_location_id :startid,
        Best_route_location_ids:responseObj. Best_route_location_ids,
        Total_uber_costs:responseObj.Total_uber_costs,
        Total_uber_duration:responseObj.Total_uber_duration,
        Total_distance:responseObj.Total_distance,
        Uber_wait_time_eta: eta,
        Status : "Requesting",
        Next_destination_location_id: responseObj.Starting_from_location_id,
     }
        getMgoSession().DB("assignment2").C("locationsdata").Update(bson.M{"_id":dataid }, bson.M{"$set": bson.M{ "status": "Requesting"}})
        nextid="complete"
        res, _ := json.Marshal(reply)
        rw.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(rw, "%s", res)
    }else{
        reply := Tstatus{
        Id:responseObj.Id,
        Starting_from_location_id :responseObj.Starting_from_location_id,
        Best_route_location_ids:responseObj. Best_route_location_ids,
        Total_uber_costs:responseObj.Total_uber_costs,
        Total_uber_duration:responseObj.Total_uber_duration,
        Total_distance:responseObj.Total_distance,
        Uber_wait_time_eta: 0 ,
        Status : "Finished",
        Next_destination_location_id: "",
     }
     getMgoSession().DB("assignment2").C("locationsdata").Update(bson.M{"_id":dataid }, bson.M{"$set": bson.M{ "status": "Finished"}})
     nextid=""
     res, _ := json.Marshal(reply)
     rw.Header().Set("Content-Type", "application/json")
     fmt.Fprintf(rw, "%s", res)
     }
}

//Function to get estimated price
func getPrice(x string, z string)(y Data){
    response, err := http.Get(x)
    if err != nil {
        return
    }
    defer response.Body.Close()
    var price []int
    resp := make(map[string]interface{})
    body, _ := ioutil.ReadAll(response.Body)
    err = json.Unmarshal(body, &resp)
    if err != nil {
        panic(err)
        return
    }
    ptr := resp["prices"].([]interface{})
    jnqquery := jsonq.NewQuery(resp)
    for i, _ := range ptr {
        prices,_ := jnqquery.Int("prices",fmt.Sprintf("%d", i),"low_estimate")
        price = append(price, prices)
    }
    min := price[0]
    for j, _ := range price {
        if(price[j]<=min && price[j]!=0){
            min = price[j]
        }
    }
    duration,_:= jnqquery.Int("prices","0","duration")
    distance,_:= jnqquery.Float("prices","0","distance")
    data := Data{
        id:z,
        price:min,
        duration:duration,
        distance:distance,
    }
    return data
}

//Function to get coordinates
func getDetails(x string) (y coordinate) {
    responseObj := Udata{}
    if err := getMgoSession().DB("assignment2").C("locations").Find(bson.M{"_id": bson.ObjectIdHex(x)}).One(&responseObj); err != nil {
        coordinates := coordinate{}
        return coordinates
    }
    detailedcoordinates := coordinate{
        lat: responseObj.Coordinate.Lat,
        lng: responseObj.Coordinate.Lng,
    }
    return detailedcoordinates
}


//Function to get details of a location
func getLocationDetails(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
    uniqueid :=  p.ByName("uniqueid")
    if !bson.IsObjectIdHex(uniqueid) {
        rw.WriteHeader(404)
        return
    }
    dataid := bson.ObjectIdHex(uniqueid)
    responseObj := Udata{}
    if err := getMgoSession().DB("assignment2").C("locations").FindId(dataid).One(&responseObj); err != nil {
        rw.WriteHeader(404)
        return
    }
    reply, _ := json.Marshal(responseObj)
    rw.Header().Set("Content-Type", "application/json")
    rw.WriteHeader(200)
    fmt.Fprintf(rw, "%s", reply)
}




// Function to create location in DataBase
func postLocation(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
    var userData Udata
    URL := "http://maps.google.com/maps/api/geocode/json?address="
    json.NewDecoder(req.Body).Decode(&userData)
    userData.Id = bson.NewObjectId()

    URL = URL + userData.Address+ " " + userData.City + " " + userData.State + " " + userData.Zip+"&sensor=false"
    URL = strings.Replace(URL, " ", "+", -1)
    response, err := http.Get(URL)
    if err != nil {  return  }
    defer response.Body.Close()

    resp := make(map[string]interface{})
    body, _ := ioutil.ReadAll(response.Body)
    err = json.Unmarshal(body, &resp)
    if err != nil {  return  }
    jasonquery := jsonq.NewQuery(resp)
    status, err := jasonquery.String("status")
    fmt.Println(status)
    if err != nil {  return  }
    if status != "OK" {
        err = errors.New(status)
        return
    }

    lat, err := jasonquery.Float("results" ,"0","geometry", "location", "lat")
    if err != nil {  return  }
    lng, err := jasonquery.Float("results", "0","geometry", "location", "lng")
    if err != nil { return }

    userData.Coordinate.Lat = lat
    userData.Coordinate.Lng = lng
    getMgoSession().DB("assignment2").C("locations").Insert(userData)
    reply, _ := json.Marshal(userData)
    rw.Header().Set("Content-Type", "application/json")
    rw.WriteHeader(201)
    fmt.Fprintf(rw, "%s", reply)
}


func main()  {
    router := httprouter.New()
    router.POST("/locations", postLocation)
    router.GET("/locations/:uniqueid", getLocationDetails)
    router.POST("/trips", findShortestRoute)
    router.GET("/trips/:tripid", getTripDetails)
    router.PUT("/trips/:tripid/request", requestTrip)
        server := http.Server{
        Addr:        "0.0.0.0:8080",
        Handler: router,
    }
    fmt.Println("Server running on port 8080")
    server.ListenAndServe()
}