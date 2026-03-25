package monycachefinal

import (
	"fmt"
	"io"
	"log"
	"mony-cache_final/consistenthash"
	"net/http"
	"net/url"
	"strings"
	"sync"
)
const defaultBasePath string="/_geecache/"
// 第一步：定义 HTTPPool 结构体（就是我们的前台接待员）
type HTTPPool struct {
	self     string     // 记录自己的地址，比如 "http://localhost:8001"
	basePath string     // 节点间通讯地址的前缀，默认是 "/_geecache/"
	mu       sync.Mutex // 互斥锁，保护将来的节点信息
	// peers：一致性哈希算法的地图（路由大脑）
	peers *consistenthash.Map 
	// httpGetters：通讯录。记录着每一个远程节点对应的那个跑腿快递员（httpGetter）
	// 比如：Key 是 "http://10.0.0.2:8001"，Value 就是去这个地址的 httpGetter
	httpGetters map[string]*httpGetter 
}

// 第二步：给外部提供一个创建 HTTPPool 的构造函数
func NewHTTPPool(self string) *HTTPPool {
	// 打印个日志，假装我们服务启动了
	log.Printf("HTTPPool is running at %s", self)
	
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}
/*
w http.ResponseWriter：这是你用来给客户端写回信（数据、状态码）的笔。
r *http.Request：这是客户端寄过来的信件（里面有请求的 URL 网址等信息）。
*/
// ServeHTTP 接收外部请求。
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter,r *http.Request){
	// 1. 安检：判断访问的路径是不是以咱们的接头暗号 (p.basePath) 开头
	if !strings.HasPrefix(r.URL.Path,p.basePath){
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}

	// 2. 打印个日志，看看别人请求了什么（方便咱们以后调试）
	log.Println("[Server] 收到请求:", r.Method, r.URL.Path)

	// 3. 拆分网址：把 "/_geecache/" 这个前缀切掉
	basePathLen := len(p.basePath)
	// r.URL.Path[basePathLen:] 的意思是：截取从前缀之后一直到最后的字符串
	// 比如 "/_geecache/scores/Tom" 就会变成 "scores/Tom"
	// 如果你用普通的 Split，遇到分隔符它就会切一刀，直到切完为止。
	//  带有 N 的 strings.SplitN(字符串, 分隔符, 份数)：限制切割次数
	// 这就是我们代码里用 SplitN 的原因。那个 N（我们传的是 2），意思是：“最多只给我切成 2 份，切出 2 份后，剩下的不管有多长、包含多少个斜杠，都不要再切了，整个保留下来！”
	parts := strings.SplitN(r.URL.Path[basePathLen:],"/",2)
	
	if len(parts) !=2 {
		http.Error(w, "bad request", http.StatusBadRequest) // 报错：客户端瞎填网址
		return
	}
	// 现在，parts[0] 就是组名(groupName)，parts[1] 就是你要找的键(key)了！

	groupName := parts[0]
	key := parts[1]
	// 1. 调用你刚写的全局函数拿档案组，填入正确的参数
	group := GetGroup(groupName)
	// 2. 判断这个组是不是不存在（在 Go 里，指针为空用什么表示？）
	if group == nil {
		// 3. 填入 404 状态码（提示：http.StatusNotFound）
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	// 1. 调用 group 的方法拿数据。它会返回两个值，记得用 := 接收
	view, err := group.Get(key)

	// 2. 判断有没有发生错误（在 Go 里，错误不为空怎么写？）
	if err != nil {
		// 3. 填入 500 服务器内部错误状态码（提示：http.StatusInternalServerError）
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 1. 设置响应头。我们约定好的键名是 "Content-Type"
	//这是一种万能型或未知型的媒体类型。它告诉浏览器：“我发给你的是一堆原始二进制数据，我也不知道具体是什么格式，或者我不希望你直接打开它。”
	w.Header().Set("Content-Type", "application/octet-stream")

	// 2. 把 view 里的字节流写进 w 里。
	// （提示：调用 view 的 ByteSlice() 方法可以拿到字节数组）
	w.Write(view.ByteSlice())
}

const defaultReplicas = 50// 虚拟节点的倍数（你可以随便定一个常量，比如 50）
func (p *HTTPPool)Set(peers ...string){
	// 1. 登记地图第一步：先关门，不许别人进来查数据（加锁保护内部状态）
	mu.Lock()
	defer mu.Unlock()
	// 2. 摊开新地图：实例化你写好的一致性哈希 Map
	// 传入虚拟节点倍数（比如 50），哈希函数传 nil（默认会用 crc32 算法）
	p.peers = consistenthash.New(defaultReplicas, nil)
	// 3. 把收到的所有节点地址，全部画到地图上（上环）
	p.peers.Add(peers...)
	// 4. 买一本新的空白通讯录（初始化 map）
	p.httpGetters = make(map[string]*httpGetter)
	// 5. 为名单上的每一个节点，招募一个专属的跑腿快递员
	for _,peer := range peers{
		p.httpGetters[peer]=&httpGetter{baseURL: peer+p.basePath}
	}
}
// PickPeer 包装了一致性哈希算法的 Get() 方法，根据具体的 key，选择出对应的节点，并返回它对应的 HTTP 客户端。
// 返回值：
// 1. PeerGetter：那个节点的专属跑腿小弟（如果返回 nil，说明不用出门）
// 2. bool：是否需要出门去别的节点拿数据（true 代表要去别人家，false 代表在自己家）
func (p *HTTPPool)PickPeer(key string)(PeerGetter,bool){
	// 1. 查地图要先加锁（防止查的时候老板刚好在更新节点名单）
	mu.Lock()
	defer mu.Unlock()
	// 2. 拿出你之前写好的哈希地图，算一算这个 key 归哪个真实节点管
	// 假设算出来叫 peer（比如是 "http://localhost:10002"）
	if peer := p.peers.Get(key);peer !="" &&peer != p.self{
		// 【极其重要的一步】
		// peer != "" 意思是地图里有节点
		// peer != p.self 意思是：算出来的这个节点，不是咱们自己！
		// 3. 既然不是咱们自己，就从通讯录里把去那个节点的专属跑腿小弟拎出来交差
		return p.httpGetters[peer],true
	}
	// 4. 兜底：如果算出来刚好是咱们自己 (peer == p.self)，或者地图是空的	
	// 那就返回 nil 和 false，告诉大堂经理：“自己去仓库找吧，别派人出门了！”
	return nil,false
}
// ---------------------------------------------------------
// 再次祭出我们的“防伪验证码”（编译期接口断言）
// 只有当 HTTPPool 完美实现了 PeerPicker 接口里的 PickPeer 方法，这行代码才不会报错
var _ PeerPicker = (*HTTPPool)(nil)
// ---------------------------------------------------------

// 这是一个内部的跑腿小弟，负责顺着网线去别的节点拿数据
type httpGetter struct {
	baseURL string // 目标节点的地址，比如 "http://localhost:10002/_geecache/"
}

func (h *httpGetter)Get(group string,key string)([]byte,error){
	// 1. 拼接他要去的那个网站地址
	// url.QueryEscape 可以防止有些客人的 key 里面有特殊符号（比如 ? 或 &）导致网址崩溃
	u :=fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(group),
		url.QueryEscape(key),
	)

	// 2. 顺着网线发起 HTTP GET 请求，去别人的前台接待员那里要数据！
	res,err := http.Get(u)
	if err != nil{
		return nil,err// 路上摔跤了（网络不通），赶紧把错误带回来
	}
	// 3. 良好习惯：不管拿没拿到，用完必须要关掉这个网络连接的门
	defer res.Body.Close()
	// 4. 判断一下对方给没给好脸（状态码是不是 200 OK）
	if res.StatusCode != http.StatusOK{
		return nil,fmt.Errorf("对方节点报错了: %v", res.Status)
	}
	// 5. 对方同意给了，咱们把信封（Body）里的东西全读出来
	bytes,err := io.ReadAll(res.Body)
	if err != nil{
		return nil, fmt.Errorf("读取对方的数据失败: %v", err)
	}
	// 6. 完美收工，带着原始数据回家交差！
	return bytes,nil
}
// 强制类型检查：如果 httpGetter 没有完美实现 PeerGetter 接口的 Get 方法，
// Go 编译器在这里就会直接报错！这是一层保险。
var _ PeerGetter = (*httpGetter)(nil)