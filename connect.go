package main

import (
    "github.com/minio/minio-go"
    "fmt"
    "encoding/json"
    "encoding/xml"
    "io/ioutil"
    "net/http"
    "strings"
    "os"
    "time"
)

type Config struct{
    AccessKeyID string `json:"access-key-id"`
    AccessKeySecret string `json:"access-key-secret"`
    Endpoint string `json:"endpoint"`
    Region string `json:"region"`
    Bucket string `json:"bucket"`
    Port int `json:"port"`
}
const DocType string = "<!DOCTYPE html>"

type HtmlContent struct{
    XMLName xml.Name `xml:"html"`
    Title string     `xml:"head>title"`
    Links []HtmlLink  `xml:"body>pre>a"`
}
type HtmlLink struct{
    XMLName xml.Name `xml:"a"`
    Href string      `xml:"href,attr"`
    Txt string       `xml:",chardata"`
}

func main(){
    if len(os.Args) != 2{
        fmt.Printf("%s <config_file>\n", os.Args[0])
        return
    }
    data, err := ioutil.ReadFile(os.Args[1])

    if err != nil{
        fmt.Println(err)
        return
    }
    var config Config

    err = json.Unmarshal(data, &config)

    if err != nil{
        fmt.Println(err)
        return
    }

    http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", config.Port), config)
}

func (config Config) ServeHTTP(w http.ResponseWriter, r *http.Request){
    path := strings.TrimLeft(r.URL.Path, "/")

    minioClient, err := minio.NewWithRegion(config.Endpoint, config.AccessKeyID, config.AccessKeySecret, true, config.Region)
    if err != nil {
        w.Write([]byte("Failed to connect to backend."))
        return
    }

    info,_ := minioClient.StatObject(config.Bucket, path, minio.StatObjectOptions{})

    if info.Size == 0{
        list := fileListing(path, config)
        w.Write([]byte(list))
    }else{
        obj, err := minioClient.GetObject(config.Bucket, path, minio.GetObjectOptions{})

        if err == nil{
            splitted := strings.Split(path, "/")
            if len(splitted) > 0{
                http.ServeContent(w, r, splitted[len(splitted)-1], time.Now(), obj)
            }
        }else{
            list := fileListing(path, config)
            w.Write([]byte(list))
        }
    }
}

func fileListing(path string, config Config) string{
    minioClient, err := minio.NewWithRegion(config.Endpoint, config.AccessKeyID, config.AccessKeySecret, true, config.Region)
    if err != nil {
        fmt.Println(err)
        return ""
    }
    doneCh := make(chan struct{})
    defer close(doneCh)

    trimmedPath := strings.TrimLeft(path, "/")

    objectCh := minioClient.ListObjects(config.Bucket, trimmedPath, false, doneCh)

    links := make([]HtmlLink, 0)

    for object := range objectCh {
        if object.Err == nil {
            tmpPath := strings.TrimPrefix(object.Key, trimmedPath)

            if len(tmpPath) > 0{
                link := HtmlLink{}
                link.Href = strings.TrimPrefix(object.Key, trimmedPath)
                link.Txt = strings.TrimPrefix(object.Key, trimmedPath)
                links = append(links, link)
            }
        }
    }
    var data []byte

    data, err = buildHtml(links, "tit")

    if err != nil{
        fmt.Println(err)
        return ""
    }

    return string(data)
}


func buildHtml(links []HtmlLink, title string)(data []byte, err error){
    var page HtmlContent
    page.Title = title
    page.Links = links

    data, err = xml.MarshalIndent(page, "\n", "\n")

    if err != nil{
        return
    }
    lines := strings.Split(string(data), "\n")
    result := DocType
    for _,line := range lines{
        trimmed := strings.Trim(line, "\n")
        if len(trimmed) > 0{
            result = result + "\n" + trimmed
        }
    }

    data = []byte(result)
    return
}
