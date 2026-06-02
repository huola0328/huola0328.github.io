package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
)

type GameState struct {
	Inventory   []string          `json:"inventory"`
	CurrentRoom string            `json:"currentRoom"`
	Rooms       map[string]Room   `json:"rooms"`
	Solved      map[string]bool   `json:"solved"`
	Message     string            `json:"message"`
}

type Room struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Items       []Item   `json:"items"`
}

type Item struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Clue        string `json:"clue,omitempty"`
}

var gameState GameState

func main() {
	initGame()

	// 静态文件
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// 页面路由
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/api/state", getState)
	http.HandleFunc("/api/action", handleAction)

	fmt.Println("🔐 密室逃脱服务器启动: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initGame() {
	gameState = GameState{
		Inventory:   []string{},
		CurrentRoom: "书桌",
		Solved:      make(map[string]bool),
		Rooms: map[string]Room{
			"书桌": {
				Name:        "💻 书桌区域",
				Description: "一张凌乱的书桌，电脑屏幕闪烁着，旁边散落着便签和照片。",
				Items: []Item{
					{Name: "便签", Description: "一张黄色便签", Clue: "上面写着：密码是室友的生日"},
					{Name: "照片", Description: "一张旧照片", Clue: "背面写着：1998-03-21"},
					{Name: "U盘", Description: "一个加密的U盘"},
				},
			},
			"书架": {
				Name:        "📚 书架区域",
				Description: "整面墙的书架，有些书被翻乱了。",
				Items: []Item{
					{Name: "日记本", Description: "带锁的日记本"},
					{Name: "书签", Description: "夹在《三体》里的书签", Clue: "书签上写着：衣柜密码 = 生日倒过来"},
				},
			},
			"衣柜": {
				Name:        "👔 衣柜区域",
				Description: "老式衣柜，铜质把手上挂着一把密码锁。",
				Items: []Item{
					{Name: "旧手机", Description: "一部老式诺基亚"},
					{Name: "信件", Description: "一封泛黄的信", Clue: "信上说：窗外有人在求救"},
				},
			},
			"窗户": {
				Name:        "🪟 窗户区域",
				Description: "窗外远处有人在用手电筒一闪一闪...（摩斯密码）",
				Items: []Item{
					{Name: "望远镜", Description: "放在窗台上的望远镜"},
				},
			},
		},
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	tmpl.Execute(w, nil)
}

func getState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gameState)
}

func handleAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
		Target string `json:"target"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	switch req.Action {
	case "look":
		gameState.Message = lookItem(req.Target)
	case "take":
		gameState.Message = takeItem(req.Target)
	case "move":
		gameState.Message = moveToRoom(req.Target)
	case "use":
		gameState.Message = useItem(req.Target)
	case "password":
		gameState.Message = tryPassword(req.Target)
	default:
		gameState.Message = "❓ 不知道要做什么"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gameState)
}

func lookItem(name string) string {
	room := gameState.Rooms[gameState.CurrentRoom]
	for _, item := range room.Items {
		if contains(item.Name, name) {
			return fmt.Sprintf("🔍 %s：%s", item.Name, item.Description)
		}
	}
	for _, itemName := range gameState.Inventory {
		if contains(itemName, name) {
			return fmt.Sprintf("🔍 背包里有 %s", itemName)
		}
	}
	return "❌ 没找到这个物品"
}

func takeItem(name string) string {
	room := gameState.Rooms[gameState.CurrentRoom]
	items := room.Items
	for i, item := range items {
		if contains(item.Name, name) {
			gameState.Inventory = append(gameState.Inventory, item.Name)
			room.Items = append(items[:i], items[i+1:]...)
			gameState.Rooms[gameState.CurrentRoom] = room

			msg := fmt.Sprintf("✅ 拿起了 %s", item.Name)
			if item.Clue != "" {
				msg += "\n💡 " + item.Clue
			}
			return msg
		}
	}
	return "❌ 这里没有这个物品"
}

func moveToRoom(name string) string {
	validRooms := []string{"书桌", "书架", "衣柜", "窗户"}
	for _, room := range validRooms {
		if contains(room, name) {
			gameState.CurrentRoom = room
			return fmt.Sprintf("🚶 走到%s", room)
		}
	}
	return "❌ 去不了那里"
}

func useItem(name string) string {
	for _, itemName := range gameState.Inventory {
		if contains(itemName, name) {
			return fmt.Sprintf("🔧 使用了 %s", itemName)
		}
	}
	return "❌ 背包里没有这个"
}

func tryPassword(pwd string) string {
	if gameState.CurrentRoom == "书桌" && pwd == "0321" {
		gameState.Solved["书桌"] = true
		gameState.Inventory = append(gameState.Inventory, "钥匙")
		return "✅ U盘解锁！掉出一把钥匙 🔑"
	}
	if gameState.CurrentRoom == "衣柜" && pwd == "2103" {
		gameState.Solved["衣柜"] = true
		return "✅ 衣柜打开！里面有旧手机和信件"
	}
	return "❌ 密码错误"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
