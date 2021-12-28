package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	uuid "github.com/satori/go.uuid"
	"github.com/xuri/excelize/v2"
)

//存放详情页的管道
var DetailUrls chan string = make(chan string)

//存放列表页的管道，用于校验所用
var listUrls chan string = make(chan string)

var wg sync.WaitGroup

//选择器结构体
type Collect struct {
	//域名
	host string
	//url
	url string
	//开始分页的下标
	startPage int
	//采集的页数
	pageNum int
	//列表选择器
	liSelector string
	//列表详情页选择器
	aSelector string
	//列表缩略图选择器
	imgSelector string
	//详情页标题选择器
	titleSelector string
	//详情页内容选择器
	contentSelector string
}

//处理列表页，返回该列表页的详情页链接切片
func (c *Collect) doList(listUrl string) (aHrefs []string) {
	resp, err := http.Get(listUrl)
	if err != nil {
		fmt.Println("http.Get(listUrl)失败:", err)
	}

	defer resp.Body.Close()

	resp.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.110 Safari/537.36")
	if resp.StatusCode != 200 {
		fmt.Println("resp.StatusCode不为200")
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Println("goquery.NewDocumentFromReader(resp.Body)失败", err)
	}

	doc.Find(c.liSelector).Each(func(i int, s *goquery.Selection) {

		aHref, ok := s.Find(c.aSelector).Attr("href")

		if ok != true {
			fmt.Println("s.Find(c.aSelector).Attr()失败")
		}

		aHrefs = append(aHrefs, aHref)
	})

	return aHrefs
}

//将该列表页得到的详情页链接存入DetailUrls管道中
func (c *Collect) writeAHrefsToChan(listUrl string) {

	aHrefs := c.doList(listUrl)

	for _, aHref := range aHrefs {
		DetailUrls <- aHref
	}

	listUrls <- listUrl

	wg.Done()
}

//从DetailUrls管道中处理详情页链接
func (c *Collect) doDetailUrl() {

	//新建一个excel文件
	f := excelize.NewFile()
	//写标题头
	f.SetCellValue("Sheet1", "A1", "title")
	f.SetCellValue("Sheet1", "B1", "img")
	f.SetCellValue("Sheet1", "C1", "content")

	//excel从第二行开始写数据，第一行要写标题头
	excelLine := 2

	//excel表名字
	excelName := uuid.NewV4().String()

	for data := range DetailUrls {
		//访问每一个详情页
		//得到标题，缩略图链接，内容
		//下载缩略图返回本地地址
		//处理内容超链接以及图片
		//将标题，图，内容，存入到excel中
		resp, err := http.Get(data)
		if err != nil {
			fmt.Println("http.Get(data)失败", err)
		}
		defer resp.Body.Close()
		doc, _ := goquery.NewDocumentFromReader(resp.Body)

		title := doc.Find(c.titleSelector).Text()
		img, _ := doc.Find(c.imgSelector).Attr("src")
		imgLocalUrl := downImg(img)
		cont, _ := doc.Find(c.contentSelector).Html()

		//将信息写入到excel中
		f.SetCellValue("Sheet1", fmt.Sprintf("A%d", excelLine), title)
		f.SetCellValue("Sheet1", fmt.Sprintf("B%d", excelLine), imgLocalUrl)
		f.SetCellValue("Sheet1", fmt.Sprintf("C%d", excelLine), cont)

		excelLine++

		fmt.Printf("标题：%s\n", title)

	}

	//保存excel文件
	if err := f.SaveAs(excelName + ".xlsx"); err != nil {
		fmt.Println(err)
	}

	fmt.Println(excelName + ".xlsx")

	wg.Done()
}

//检验DetailUrls管道是否写入完毕
func (c *Collect) checkOK() {
	var count int
	for {
		<-listUrls

		count++
		//如果所有列表页的记录都在listUrls管道里就关闭DetailUrls，退出检验
		if count == c.pageNum {
			close(DetailUrls)
			break
		}
	}

	wg.Done()
}

//下载缩略图并返回真实图片地址
func downImg(imgUrl string) string {

	resp, err := http.Get(imgUrl)

	if err != nil {
		fmt.Println("http.Get()请求图片失败", err)
	}

	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		fmt.Println("ioutil.ReadAll(resp.Body)", err)
	}

	//随机一个图片名称
	id := uuid.NewV4().String()

	sonDir := time.Now().Format("2006-01-02")

	dir := "uploads/" + sonDir

	_, err = os.Stat(dir)

	//文件目录存在
	if os.IsNotExist(err) {

		fmt.Println("文件目录不存在,系统将自动新建目录")
		//创建目录包括子目录 os.ModePerm代表最高权限
		os.MkdirAll(dir, os.ModePerm)

	}

	err = ioutil.WriteFile(dir+"/"+id+".png", bytes, 0666)

	if err != nil {
		fmt.Println("ioutil.WriteFile写入文件失败", err)
	}

	return dir + "/" + id + ".png"

}

func main() {

	collect := &Collect{
		host:       "https://www.522gg.com",
		url:        "/game/0_0_0_0_0_[PAGE].html",
		startPage:  1,
		pageNum:    50,
		liSelector: "body > div.w1200 > div.box2 > div.flex > div.fr > div > div.bod > div",
		aSelector:  " div > a",
		//详情页缩略图
		imgSelector: "body > div.w1200 > div.flex > div.left > div.article_game1 > div.img > img",
		//详情页标题
		titleSelector: "body > div.w1200 > div.flex > div.left > div.article_game1 > div.info > h1",
		//详情页内容
		contentSelector: "body > div.w1200 > div.flex > div.left > div.article_game3 > div",
	}

	//写入详情页链接
	for i := collect.startPage; i <= collect.pageNum; i++ {

		realUrl := strings.Replace(collect.host+collect.url, "[PAGE]", strconv.Itoa(i), 1)

		wg.Add(1)

		go collect.writeAHrefsToChan(realUrl)

	}

	// //校验监控
	wg.Add(1)
	go collect.checkOK()

	//输出详情页链接 多协程会保存为多份excel，用一个协程就行
	// for i := 0; i < 5; i++ {
	// 	wg.Add(1)
	// 	go collect.doDetailUrl()
	// }

	//启动一个协程将每个详情页里的内容写道excel中
	wg.Add(1)
	go collect.doDetailUrl()

	wg.Wait()

}
