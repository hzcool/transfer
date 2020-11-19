package main

import (
	"Transfer/common"
	"encoding/json"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

type MyFileInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
	IsDir     bool   `json:"is_dir"`
	Type      string `json:"type"`

}
type MyFileInfos []*MyFileInfo
func (fi MyFileInfos) Len() int {
	return len(fi)
}
func (fi MyFileInfos) Less(i, j int) bool {
	if fi[i].IsDir == fi[j].IsDir {
		if fi[i].Type == fi[j].Type {
			return fi[i].Name < fi[j].Name
		}
		return fi[i].Type < fi[j].Type
	}
	if fi[i].IsDir {
		return true
	}
	return false
}
func (fi MyFileInfos) Swap(i, j int) {
	fi[i], fi[j] = fi[j], fi[i]
}

const prefix = "static/"
const tmpDir = "tmp/"

var mt sync.Mutex
var expire time.Duration //token 过期时间
var pwd string //操作密码验证

func init()  {
	content,err := common.GetContent("config.json");
	if err!=nil {
		panic(err)
	}
	var cfg map[string]interface{}
	err = json.Unmarshal([]byte(content),&cfg)
	if err!=nil {
		panic(err)
	}
	var ok1,ok2 bool
	var minute float64
	pwd,ok1 = cfg["pwd"].(string)
	minute,ok2 = cfg["expire"].(float64)
	if !ok1 || !ok2 {
		panic("load config.json error")
	}
	expire = time.Minute * time.Duration(int(minute))
}

func authToken(c *gin.Context)  { //token验证
	session := sessions.Default(c)
	token := session.Get("token")
	if token==nil || !common.ExistToken(token.(string)) {
		c.String(401,"需要密码验证")
		c.Abort()
	}
}
func checkLogin(c *gin.Context)  {
	c.String(200,"ok")
}

func checkPwd(c *gin.Context)  {
	if c.PostForm("pwd")!=pwd {
		c.String(403,"密码错误")
		return
	}
	token := common.GetNewToken(expire)
	session := sessions.Default(c)
	session.Set("token",token)
	session.Save()
	c.String(200,"ok")
}


func submit(c *gin.Context) {
	fileName := c.Query("name")
	offset, _ := strconv.ParseInt(c.Query("offset"), 10, 64)
	siz, _ := strconv.ParseInt(c.Query("size"), 10, 64)
	path := filepath.Join(prefix, fileName)
	mt.Lock()
	dir := filepath.Dir(path)
	if !common.IsDir(dir) {
		os.MkdirAll(filepath.Dir(path), os.ModePerm)
	}
	if exist, _ := common.Exists(path); !exist {
		os.Create(path)
	}
	mt.Unlock()
	file2, _, err := c.Request.FormFile("blob")
	if err != nil {
		c.String(403, err.Error())
		return
	}
	var buf = make([]byte, siz+5)
	n, _ := file2.Read(buf)
	buf = buf[0:n]
	file, _ := os.OpenFile(path, os.O_WRONLY, 0666)
	defer file.Close()
	if _, err := file.WriteAt(buf, offset); err != nil {
		c.String(403, err.Error())
		return
	}
	c.String(200, "ok")
}
func showDir(c *gin.Context) {
	path := filepath.Join(prefix, c.Query("path"))
	if !common.IsDir(path) {
		c.String(403, "Not Found")
		return
	}
	rd, _ := ioutil.ReadDir(path)
	files := make(MyFileInfos, len(rd))
	for i, fi := range rd {
		files[i] = &MyFileInfo{
			Name:      fi.Name(),
			Size:      fi.Size(),
			CreatedAt: fi.ModTime().Format("2006-01-02 15:04:05"),
			IsDir:     fi.IsDir(),
			Type:      "folder",
		}
		if !fi.IsDir() {
			files[i].Type = filepath.Ext(fi.Name())
			if(files[i].Type=="") {
				files[i].Type = "bin"
			}
			if(files[i].Type[0]=='.') {
				files[i].Type = files[i].Type[1:]
			}
		}
	}
	sort.Sort(files)
	c.JSON(200, files)
}
func mkdir(c *gin.Context) {
	path := filepath.Join(prefix, c.Query("path"))
	if exist, _ := common.Exists(path); exist {
		c.String(403, "File or Director Existed")
		return
	}
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		c.String(403, err.Error())
		return
	}
	c.String(200, "ok")
}
func delete(c *gin.Context) {
	path := filepath.Join(prefix, c.Query("path"))
	if exist, _ := common.Exists(path); !exist {
		c.String(403, "Not Found")
		return
	}
	if err := os.RemoveAll(path); err != nil {
		c.String(403, err.Error())
		return
	}
	c.String(200, "ok")
}
func lookup(c *gin.Context) {
	path := filepath.Join(prefix, c.Query("path"))
	if exist, _ := common.Exists(path); !exist {
		c.String(403, "Not Found")
		return
	}
	if common.GetFileSize(path) > 1024*1024*5 {
		c.String(403, "文件超过5MB,读取失败")
		return
	}
	content, err := common.GetContent(path)
	if err != nil {
		c.String(403, err.Error())
	}
	c.String(200, content)
}
func download(c *gin.Context) {
	path := filepath.Join(prefix, c.Query("path"))
	if exist, _ := common.Exists(path); !exist {
		c.String(403, "Not Found")
		return
	}
	if common.IsDir(path) {
		destZip := filepath.Join(tmpDir, "data-"+strconv.FormatInt(time.Now().Unix(), 10)+".zip")
		defer os.Remove(destZip)
		if err := common.CompressDir(path, destZip); err != nil {
			c.String(403, err.Error())
			return
		}
		c.Writer.Header().Set("Content-Type", "application/zip")
		c.File(destZip)
	} else {
		c.Writer.Header().Set("Content-Type", "application/octet-stream")
		c.File(path)
	}
}
func reName(c *gin.Context)  {
	path := filepath.Join(prefix, c.Query("path"),c.Query("orgName"))
	if exist, _ := common.Exists(path); !exist {
		c.String(403, "Not Found")
		return
	}
	newName := filepath.Join(prefix,c.Query("path"),c.Query("newName")+filepath.Ext(path))
	if err:=os.Rename(path,newName);err!=nil {
		c.String(403,err.Error())
		return
	}
	c.String(200,"ok")
}
func initRouters(r *gin.Engine)  {
	r.LoadHTMLFiles("./dist/index.html")
	r.StaticFS("/js", http.Dir("./dist/js"))
	r.StaticFS("/css", http.Dir("./dist/css"))
	r.StaticFS("/fonts", http.Dir("./dist/fonts"))
	r.StaticFS("/img", http.Dir("./dist/img"))
	r.StaticFile("favicon.ico", "./dist/favicon.ico")
	r.StaticFS("/static",http.Dir("./static"))
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})
	store := cookie.NewStore([]byte("secret")) //启用cookie和session
	store.Options(sessions.Options{
		MaxAge: int(expire.Seconds()), //过期时间
	})
	r.Use(sessions.Sessions("ginSession", store))

	r.POST("checkPwd",checkPwd)
	r.POST("submit", submit)
	g := r.Group("/")
	g.Use(authToken)
	{
		g.GET("checkLogin",checkLogin)
		g.GET("showDir", showDir)
		g.GET("mkdir", mkdir)
		g.GET("delete", delete)
		g.GET("lookup", lookup)
		g.GET("download", download)
		g.GET("reName", reName)
	}
}

func main() {
	//r := gin.Default()
	r := gin.New()
	r.Use(gin.Recovery())
	initRouters(r)
	r.Run(":3000")
}
