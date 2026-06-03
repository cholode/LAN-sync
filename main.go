package main

import (
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	//"io"
	"lan-im-go/api"
	"lan-im-go/config"
	"lan-im-go/core"
	"lan-im-go/infrastructure"
	"lan-im-go/internal/archiver"
	"lan-im-go/middleware"
	"lan-im-go/repository"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {

	//  鍗曠嫭鍚姩涓€涓?goroutine 鐩戝惉 6060 绔彛锛堜笉褰卞搷涓讳笟鍔★級
	go func() {
		// 鍦板潃锛?.0.0.0:6060 鍏佽澶栭儴/瀹夸富鏈鸿闂?
		err := http.ListenAndServe("0.0.0.0:6060", nil)
		if err != nil {
			panic("pprof start failed: " + err.Error())
		}
	}()
	//log.SetOutput(io.Discard)
	// ========================================================================
	// 闃舵1锛氱幆澧冧笌鍩虹璁炬柦鍒濆鍖?
	// ========================================================================
	// 浠庣幆澧冨彉閲忚鍙栨暟鎹簱閰嶇疆锛屼负绌烘椂浣跨敤鏈湴榛樿閰嶇疆锛堥€傞厤鏈湴璋冭瘯锛?
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "root:123456@tcp(127.0.0.1:3306)/lan_im?charset=utf8mb4&parseTime=True&loc=Local"
		log.Println("[璀﹀憡] 鏈娴嬪埌DB_DSN鐜鍙橀噺锛屼娇鐢ㄦ湰鍦伴粯璁ら厤缃繛鎺ySQL")
	}

	//涓棿浠跺垵濮嬪寲锛宺edis锛宬afka
	config.InitRedis()
	config.InitKafka()

	defer config.RedisClient.Close()
	defer config.KafkaProducer.Close()

	// 鍒濆鍖栨暟鎹簱杩炴帴姹犲苟鑷姩鍚屾琛ㄧ粨鏋?
	// 鏁版嵁搴撹繛鎺ュけ璐ユ椂绋嬪簭鐩存帴缁堟锛屼繚璇佹湇鍔″惎鍔ㄥ畬鏁存€?
	infrastructure.InitDatabase(dsn)
	api.InitFileDirs()
	// ========================================================================
	// 闃舵2锛氭暟鎹闂眰鍒濆鍖?
	// ========================================================================
	// 娉ㄥ叆鏁版嵁搴撹繛鎺ュ疄渚嬪埌鏁版嵁璁块棶灞?
	// 涓氬姟閫昏緫缁熶竴閫氳繃鏁版嵁璁块棶灞傛帴鍙ｆ搷浣滄暟鎹簱
	repository.InitRepositories(infrastructure.DB)
	log.Println("[灏辩华] 鏁版嵁璁块棶灞傚垵濮嬪寲瀹屾垚")

	repository.InitRepositories(infrastructure.DB)
	log.Println("[系统就绪] 数据访问层(DAL) 初始化完成")

	// ========================================================================
	// 闃舵 3锛氬悗鍙扮ǔ鎬佹秷璐硅€呭敜閱?(Kafka Consumer Daemon)
	// ========================================================================
	kafkaAddrStr := os.Getenv("KAFKA_ADDR")
	if kafkaAddrStr == "" {
		kafkaAddrStr = "127.0.0.1:9092" // 鏈湴闄嶇骇
	}

	// 缁勮娑堣垂鑰咃紝瀹冨唴閮ㄤ細鑷姩璋冪敤 repository.Message 杩涜鐗╃悊钀界洏
	worker := archiver.NewWorker([]string{kafkaAddrStr}, "im_chat_messages", "mysql_archiver_group")

	// 鍒涘缓鍏ㄥ眬鐢熷懡鍛ㄦ湡涓婁笅鏂?
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		log.Println("[绯荤粺鎸囦护] Kafka 绋虫€佹秷璐瑰崗绋嬭繘鍏ユ寰幆鐩戝惉...")
		worker.Start(ctx)
	}()

	// ========================================================================
	// 闃舵3锛歐ebSocket鏍稿績寮曟搸鍚姩
	// ========================================================================
	// 鍒涘缓鍏ㄥ眬WebSocket璺敱涓績
	hub := core.NewHub()
	// 鍚姩寮曟搸鐩戝惉璋冨害閫氶亾
	go hub.Run(ctx)
	go core.StartGlobalListener(ctx, hub)
	log.Println("[灏辩华] WebSocket鏍稿績寮曟搸鍚姩瀹屾垚")

	// ========================================================================
	// 闃舵4锛欻TTP鏈嶅姟涓庤矾鐢遍厤缃?
	// ========================================================================
	// 寮€鍙戠幆澧冧娇鐢ㄩ粯璁ゆā寮忥紝鐢熶骇鐜寤鸿鍒囨崲涓哄彂甯冩ā寮?
	//r := gin.Default()
	//gin.SetMode(gin.ReleaseMode)
	gin.SetMode(gin.DebugMode)
	r := gin.New()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	pprof.Register(r)
	// ========================================================================
	// 璺ㄥ煙閰嶇疆锛堥渶鍦ㄨ矾鐢辨敞鍐屽墠閰嶇疆锛?
	// ========================================================================
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true, // 寮€鍙戠幆澧冨厑璁告墍鏈夊煙鍚嶏紝鐢熶骇鐜闇€閰嶇疆鎸囧畾鍓嶇鍩熷悕
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour, // 棰勬璇锋眰缂撳瓨鏃堕暱锛屽噺灏戦噸澶嶈姹?
	}))

	// ========================================================================
	// 璺敱鍒嗙粍閰嶇疆
	// ========================================================================

	// 鍏叡璺敱缁勶細鏃犻渶韬唤楠岃瘉
	public := r.Group("/api/v1")
	{
		// 寮€鏀炬帴鍙ｏ細鐢ㄦ埛娉ㄥ唽銆佺櫥褰?
		public.POST("/register", api.RegisterHandler)
		public.POST("/login", api.LoginHandler)
		// 鏂囦欢涓嬭浇锛氳矾寰勪腑鍚暣鏂囦欢 SHA-256 鍓嶇紑锛岃涓鸿兘鍔涢摼鎺ワ紱缇ゆ垚鍛樻棤闇€鍦?URL 涓甫鍚勮嚜 JWT 鍗冲彲鍦ㄦ祻瑙堝櫒涓墦寮€涓嬭浇
		public.GET("/download/*filepath", api.DownloadFile)
	}

	// 閴存潈璺敱缁勶細闇€JWT韬唤楠岃瘉
	authorized := r.Group("/api/v1")
	// 娉ㄥ唽JWT韬唤楠岃瘉涓棿浠?
	authorized.Use(middleware.JWTAuth())
	{
		// WebSocket杩炴帴鍏ュ彛
		log.Printf("杩涘叆WebSocket杩炴帴閰嶇疆\n")
		authorized.GET("/ws", func(c *gin.Context) {
			api.WsEndpoint(hub)(c)
		})

		// 鏂囦欢涓婁紶鐩稿叧鎺ュ彛
		authorized.GET("/upload/status", api.CheckUploadStatus)
		authorized.POST("/upload/chunk", api.UploadChunk)
		authorized.POST("/upload/merge", api.MergeChunks)
		// 缇よ亰鐩稿叧鎺ュ彛
		authorized.GET("/rooms/:id/messages", api.GetChatHistory())
		authorized.POST("/rooms/:id/join", api.JoinRoom(hub))
		authorized.GET("/rooms/:id/members", api.GetRoomMembers())
		authorized.DELETE("/rooms/:id/members/:user_id", api.RemoveRoomMember(hub))
		authorized.DELETE("/rooms/:id/disband", api.OwnerDisbandRoom(hub))
		authorized.DELETE("/upload/cancel", api.CancelUpload)
		// 缇よ亰绠＄悊鎺ュ彛
		authorized.POST("/rooms", api.CreateRoom(hub))
		authorized.GET("/my_rooms", api.GetMyRooms())
	}

	// 绠＄悊鍛樿矾鐢辩粍锛氶渶绠＄悊鍛樻潈闄?
	admin := r.Group("/api/v1/admin")
	// 涓棿浠舵墽琛岄『搴忥細鍏堣韩浠介獙璇侊紝鍐嶆潈闄愭牎楠?
	admin.Use(middleware.JWTAuth(), middleware.SuperAdminOnly())
	{
		// 绠＄悊鍛樻搷浣滄帴鍙?
		admin.DELETE("/users/:id", api.AdminDeleteUser(hub))
		admin.DELETE("/rooms/:id", api.AdminDeleteRoom(hub))
	}

	// ========================================================================
	// 闃舵5锛氬惎鍔℉TTP鏈嶅姟
	// ========================================================================
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,                // 灏?Gin 璺敱寮曟搸寮鸿鎸傝浇鍒板簳灞?Server
		ReadTimeout:       5 * time.Second,  // 璇诲彇瀹屾暣璇锋眰澶?浣撶殑鏈€闀挎椂闂?
		ReadHeaderTimeout: 3 * time.Second,  // 闃插尽 Slowloris 鎱㈤€熷ご閮ㄦ敾鍑?
		WriteTimeout:      10 * time.Second, // 鍝嶅簲鍐欏洖鐨勬渶闀挎椂闂?
		IdleTimeout:       15 * time.Second, // 搴曞眰 TCP Keep-Alive 绌洪棽鏈€澶у瓨娲绘椂闂?
	}

	log.Printf("[鏋舵瀯灏辩华] LAN-IM 鏈嶅姟绔惎鍔ㄦ垚鍔燂紝鐩戝惉绔彛 :%s", port)

	// 鎵ц甯︽湁寮傚父鎹曡幏鐨勫簳灞傚惎鍔ㄨ皟鐢?
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[鑷村懡閿欒] 缃戝叧宕╂簝: %v", err)
	}

	cancel()
}
