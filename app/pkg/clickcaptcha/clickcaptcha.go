package clickcaptcha

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/image/font"
	"gorm.io/gorm"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

// 可以使用的背景图片路径
var bgPaths []string = []string{
	"/static/images/captcha/click/bgs/1.png",
	"/static/images/captcha/click/bgs/2.png",
	"/static/images/captcha/click/bgs/2.png",
}

// 可以使用的字体文件路径
var fontPaths []string = []string{
	"/static/fonts/zhttfs/SourceHanSansCN-Normal.ttf",
}

// 验证点 Icon 映射表
var iconDict map[string]string = map[string]string{
	"aeroplane": "飞机",
	"apple":     "苹果",
	"banana":    "香蕉",
	"bell":      "铃铛",
	"bicycle":   "自行车",
	"bird":      "小鸟",
	"bomb":      "炸弹",
	"butterfly": "蝴蝶",
	"candy":     "糖果",
	"crab":      "螃蟹",
	"cup":       "杯子",
	"dolphin":   "海豚",
	"fire":      "火",
	"guitar":    "吉他",
	"hexagon":   "六角形",
	"pear":      "梨",
	"rocket":    "火箭",
	"sailboat":  "帆船",
	"snowflake": "雪花",
	"wolf head": "狼头",
}

// 配置
type Config struct {
	Alpha         float64 // 透明度
	ZhSet         string  // 中文字符集
	Mode          string
	Length        int
	ConfuseLength int
}

type ClickCaptcha struct {
	sqlDB  *gorm.DB
	config Config
}

func NewCaptcha(config *conf.Configuration, sqlDB *gorm.DB) *ClickCaptcha {
	defaultConfig := Config{
		Mode:          config.ClickCaptcha.Mode,
		Length:        config.ClickCaptcha.Length,
		ConfuseLength: config.ClickCaptcha.ConfuseLength,
		Alpha:         36,
		ZhSet:         "们以我到他会作时要动国产的一是工就年阶义发成部民可出能方进在了不和有大这主中人上为来分生对于学下级地个用同行面说种过命度革而多子后自社加小机也经力线本电高量长党得实家定深法表着水理化争现所二起政三好十战无农使性前等反体合斗路图把结第里正新开论之物从当两些还天资事队批点育重其思与间内去因件日利相由压员气业代全组数果期导平各基或月毛然如应形想制心样干都向变关问比展那它最及外没看治提五解系林者米群头意只明四道马认次文通但条较克又公孔领军流入接席位情运器并飞原油放立题质指建区验活众很教决特此常石强极土少已根共直团统式转别造切九你取西持总料连任志观调七么山程百报更见必真保热委手改管处己将修支识病象几先老光专什六型具示复安带每东增则完风回南广劳轮科北打积车计给节做务被整联步类集号列温装即毫知轴研单色坚据速防史拉世设达尔场织历花受求传口断况采精金界品判参层止边清至万确究书术状厂须离再目海交权且儿青才证低越际八试规斯近注办布门铁需走议县兵固除般引齿千胜细影济白格效置推空配刀叶率述今选养德话查差半敌始片施响收华觉备名红续均药标记难存测士身紧液派准斤角降维板许破述技消底床田势端感往神便贺村构照容非搞亚磨族火段算适讲按值美态黄易彪服早班麦削信排台声该击素张密害侯草何树肥继右属市严径螺检左页抗苏显苦英快称坏移约巴材省黑武培著河帝仅针怎植京助升王眼她抓含苗副杂普谈围食射源例致酸旧却充足短划剂宣环落首尺波承粉践府鱼随考刻靠够满夫失包住促枝局菌杆周护岩师举曲春元超负砂封换太模贫减阳扬江析亩木言球朝医校古呢稻宋听唯输滑站另卫字鼓刚写刘微略范供阿块某功套友限项余倒卷创律雨让骨远帮初皮播优占死毒圈伟季训控激找叫云互跟裂粮粒母练塞钢顶策双留误础吸阻故寸盾晚丝女散焊功株亲院冷彻弹错散商视艺灭版烈零室轻血倍缺厘泵察绝富城冲喷壤简否柱李望盘磁雄似困巩益洲脱投送奴侧润盖挥距触星松送获兴独官混纪依未突架宽冬章湿偏纹吃执阀矿寨责熟稳夺硬价努翻奇甲预职评读背协损棉侵灰虽矛厚罗泥辟告卵箱掌氧恩爱停曾溶营终纲孟钱待尽俄缩沙退陈讨奋械载胞幼哪剥迫旋征槽倒握担仍呀鲜吧卡粗介钻逐弱脚怕盐末阴丰雾冠丙街莱贝辐肠付吉渗瑞惊顿挤秒悬姆烂森糖圣凹陶词迟蚕亿矩康遵牧遭幅园腔订香肉弟屋敏恢忘编印蜂急拿扩伤飞露核缘游振操央伍域甚迅辉异序免纸夜乡久隶缸夹念兰映沟乙吗儒杀汽磷艰晶插埃燃欢铁补咱芽永瓦倾阵碳演威附牙芽永瓦斜灌欧献顺猪洋腐请透司危括脉宜笑若尾束壮暴企菜穗楚汉愈绿拖牛份染既秋遍锻玉夏疗尖殖井费州访吹荣铜沿替滚客召旱悟刺脑措贯藏敢令隙炉壳硫煤迎铸粘探临薄旬善福纵择礼愿伏残雷延烟句纯渐耕跑泽慢栽鲁赤繁境潮横掉锥希池败船假亮谓托伙哲怀割摆贡呈劲财仪沉炼麻罪祖息车穿货销齐鼠抽画饲龙库守筑房歌寒喜哥洗蚀废纳腹乎录镜妇恶脂庄擦险赞钟摇典柄辩竹谷卖乱虚桥奥伯赶垂途额壁网截野遗静谋弄挂课镇妄盛耐援扎虑键归符庆聚绕摩忙舞遇索顾胶羊湖钉仁音迹碎伸灯避泛亡答勇频皇柳哈揭甘诺概宪浓岛袭谁洪谢炮浇斑讯懂灵蛋闭孩释乳巨徒私银伊景坦累匀霉杜乐勒隔弯绩招绍胡呼痛峰零柴簧午跳居尚丁秦稍追梁折耗碱殊岗挖氏刃剧堆赫荷胸衡勤膜篇登驻案刊秧缓凸役剪川雪链渔啦脸户洛孢勃盟买杨宗焦赛旗滤硅炭股坐蒸凝竟陷枪黎救冒暗洞犯筒您宋弧爆谬涂味津臂障褐陆啊健尊豆拔莫抵桑坡缝警挑污冰柬嘴啥饭塑寄赵喊垫丹渡耳刨虎笔稀昆浪萨茶滴浅拥穴覆伦娘吨浸袖珠雌妈紫戏塔锤震岁貌洁剖牢锋疑霸闪埔猛诉刷狠忽灾闹乔唐漏闻沈熔氯荒茎男凡抢像浆旁玻亦忠唱蒙予纷捕锁尤乘乌智淡允叛畜俘摸锈扫毕璃宝芯爷鉴秘净蒋钙肩腾枯抛轨堂拌爸循诱祝励肯酒绳穷塘燥泡袋朗喂铝软渠颗惯贸粪综墙趋彼届墨碍启逆卸航衣孙龄岭骗休借",
	}
	return &ClickCaptcha{config: defaultConfig, sqlDB: sqlDB}
}

type BaCaptcha struct {
	Key        string `gorm:"column:key;primaryKey;comment:验证码Key" json:"key"`    // 验证码Key
	Code       string `gorm:"column:code;not null;comment:验证码(加密后)" json:"code"`  // 验证码(加密后)
	Captcha    string `gorm:"column:captcha;comment:验证码数据" json:"captcha"`        // 验证码数据
	CreateTime int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"` // 创建时间
	ExpireTime int64  `gorm:"column:expire_time;comment:过期时间" json:"expire_time"` // 过期时间
}

type Point struct {
	Size   int  //字体大小
	Icon   bool //是否图标
	Name   string
	Text   string
	Width  int //图标或文字宽
	Height int //图标或文字高
	X      int //位置坐标
	Y      int
}

type Captcha struct {
	Width    int //背景图宽
	Height   int //背景图高
	PointArr []*Point
}

/**
 * 创建图形验证码
 * id 验证码ID，开发者自定义
 * 返回验证码图片的base64编码和验证码文字信息
 * TODO:解决重叠,重复
 */
func (c *ClickCaptcha) Create(ctx *gin.Context, id string) (map[string]interface{}, error) {
	rand.Seed(time.Now().UnixNano())
	randIndex := rand.Intn(len(bgPaths) - 1)
	imagePath := bgPaths[randIndex]
	bgImg, err := loadImage(utils.RootPath() + imagePath)
	if err != nil {
		return nil, err
	}
	imgWidth, imgHeight := bgImg.Bounds().Dx(), bgImg.Bounds().Dy()

	// 加载字体文件
	fontPath := fontPaths[0]
	fontBytes, err := os.ReadFile(utils.RootPath() + fontPath)
	if err != nil {
		return nil, err
	}
	fontData, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return nil, err
	}

	currentLang := ctx.GetHeader("think-lang")

	iconImgMap := map[string]image.Image{}
	pointArr := []*Point{}
	randPoints := c.randPoints(c.config.Length + c.config.ConfuseLength)
	for _, v := range randPoints {
		point := Point{
			Size: rand.Intn(16) + 15,
		}

		if _, ok := iconDict[v]; ok {
			//图标
			point.Icon = true
			point.Name = v
			point.Text = "<" + v + ">"
			if currentLang == "zh-cn" {
				point.Text = "<" + iconDict[v] + ">"
			}

			iconImg, err := loadImage(utils.RootPath() + "/static/images/captcha/click/icons/" + v + ".png")
			if err != nil {
				return nil, err
			}
			iconImgMap[v] = iconImg
			point.Width, point.Height = iconImg.Bounds().Dx(), iconImg.Bounds().Dy()
		} else {
			//字符串文本框宽度和长度
			point.Icon = false
			point.Text = v
			point.Width, point.Height, err = getFontWidthAndHeight(v, point.Size, fontBytes)
			if err != nil {
				return nil, err
			}
		}
		pointArr = append(pointArr, &point)
	}
	texts := []string{}
	// 随机生成验证点位置
	for _, v := range pointArr {
		v.X, v.Y = c.RandPosition(pointArr, imgWidth, imgHeight, v.Width, v.Height, v.Icon)
		texts = append(texts, v.Text)
	}

	// 创建一个新画布复制原图，以便绘制
	drawImg := image.NewRGBA(bgImg.Bounds())
	draw.Draw(drawImg, drawImg.Bounds(), bgImg, image.Point{}, draw.Src)
	for _, v := range pointArr {
		if v.Icon {
			draw.Draw(drawImg, iconImgMap[v.Name].Bounds().Add(image.Point{v.X, v.Y}), iconImgMap[v.Name], image.Point{}, draw.Over)
		} else {
			// 设置颜色
			// color := color.RGBA{239, 239, 234, uint8(127 - c.config.Alpha*(float64(127)/100))}
			pt := freetype.Pt(int(v.X), int(v.Y))
			fctx := freetype.NewContext()
			fctx.SetFont(fontData)
			fctx.SetFontSize(float64(v.Size))
			fctx.SetClip(drawImg.Bounds())
			fctx.SetDst(drawImg)
			fctx.SetSrc(image.NewUniform(color.White))
			fctx.DrawString(v.Text, pt)
			fctx.SetHinting(font.HintingFull)
		}
	}

	var buf bytes.Buffer
	err = png.Encode(&buf, drawImg)
	if err != nil {
		return nil, err
	}
	content := buf.Bytes()

	texts = texts[0:c.config.Length]
	captcha := Captcha{
		Width:    imgWidth,
		Height:   imgHeight,
		PointArr: pointArr[0:c.config.Length],
	}
	captchaStr, _ := json.Marshal(captcha)

	key := utils.Md5(id)
	var result map[string]interface{}
	c.sqlDB.Table("ba_captcha").Where(" `key` = ? ", key).Scan(&result)

	if _, ok := result["key"]; ok {
		err = c.sqlDB.Table("ba_captcha").Where("`key`=?", key).Updates(map[string]interface{}{
			"code":        utils.Md5(strings.Join(texts, ",")),
			"captcha":     captchaStr,
			"create_time": time.Now().Unix(),
			"expire_time": time.Now().Unix() + 600,
		}).Error
	} else {
		err = c.sqlDB.Table("ba_captcha").Create(map[string]interface{}{
			"key":         key,
			"code":        utils.Md5(strings.Join(texts, ",")),
			"captcha":     captchaStr,
			"create_time": time.Now().Unix(),
			"expire_time": time.Now().Unix() + 600,
		}).Error
	}

	return map[string]interface{}{
		"id":     id,
		"text":   texts,
		"base64": fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(content)),
		"width":  imgWidth,
		"height": imgHeight,
	}, err
}

func getFontWidthAndHeight(text string, fontSize int, fontBytes []byte) (int, int, error) {
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		return 0, 0, err
	}
	face := truetype.NewFace(f, &truetype.Options{
		Size:    float64(fontSize),
		Hinting: font.HintingFull,
	})

	bounds, _ := font.BoundString(face, text)
	// 输出文本尺寸
	textWidth := bounds.Max.X.Ceil() - bounds.Min.X.Floor()
	textHeight := bounds.Max.Y.Ceil() - bounds.Min.Y.Floor()
	return int(textWidth), int(textHeight), nil
}

func loadImage(filePath string) (image.Image, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(fileData)
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}
	return img, nil
}

/**
 * 检查验证码
 * 开发者自定义的验证码ID
 * info  验证信息201,61-274,92;350;200
 * unset 验证成功是否删除验证码
 */
func (c *ClickCaptcha) Check(id string, info string, unset bool) bool {
	key := utils.Md5(id)

	baCaptcha := BaCaptcha{}
	err := c.sqlDB.Table("ba_captcha").Where("`key`=?", key).First(&baCaptcha).Error
	if err != nil {
		return false
	}

	if baCaptcha.ExpireTime < time.Now().Unix() {
		c.sqlDB.Table("ba_captcha").Where("`key`=?", key).Delete(nil)
		return false
	}

	captcha := Captcha{}
	err = json.Unmarshal([]byte(baCaptcha.Captcha), &captcha)
	if err != nil {
		return false
	}

	infoArr := strings.Split(info, ";")
	xyArr := strings.Split(infoArr[0], "-")
	w, _ := strconv.Atoi(infoArr[1])
	h, _ := strconv.Atoi(infoArr[2])
	xPro := w / captcha.Width
	yPro := h / captcha.Height

	for k, v := range xyArr {
		xy := strings.Split(v, ",")
		x, _ := strconv.Atoi(xy[0])
		y, _ := strconv.Atoi(xy[1])
		if x/xPro < captcha.PointArr[k].X || x/xPro > captcha.PointArr[k].X+captcha.PointArr[k].Width {
			return false
		}

		phStart := captcha.PointArr[k].Y - captcha.PointArr[k].Height
		phEnd := captcha.PointArr[k].Y
		if captcha.PointArr[k].Icon {
			phStart = captcha.PointArr[k].Y
			phEnd = captcha.PointArr[k].Y + captcha.PointArr[k].Height
		}
		if y/yPro < phStart || y/yPro > phEnd {
			return false
		}
	}

	if unset {
		c.sqlDB.Table("ba_captcha").Where("`key`=?", key).Delete(nil)
	}
	return true
}

// 随机生成验证点元素
func (c *ClickCaptcha) randPoints(length int) []string {
	arr := []string{}
	rand.Seed(time.Now().UnixNano())
	if strings.Contains(c.config.Mode, "text") {
		runes := []rune(c.config.ZhSet)
		for i := 0; i < length; i++ {
			randomIndex := rand.Intn(len(runes))
			arr = append(arr, string(runes[randomIndex]))
		}
	}
	if strings.Contains(c.config.Mode, "icon") {
		icons := []string{}
		for key := range iconDict {
			icons = append(icons, key)
		}
		for i := 0; i < length; i++ {
			randomIndex := rand.Intn(len(icons))
			arr = append(arr, string(icons[randomIndex]))
		}
	}
	for i := len(arr) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		arr[i], arr[j] = arr[j], arr[i]
	}
	return arr[:length]
}

// 随机生成位置布局
func (c *ClickCaptcha) RandPosition(poinArr []*Point, imgW int, imgH int, w int, h int, isIcon bool) (x, y int) {
	rand.Seed(time.Now().UnixNano())
	x = rand.Intn(imgW - w)
	y = rand.Intn(imgH-2*h) + h

	if !c.CheckPosition(poinArr, x, y, w, h, isIcon) {
		x, y = c.RandPosition(poinArr, imgW, imgH, w, h, isIcon)
	}
	return x, y
}

/**
 * 碰撞验证
 * pointArr 验证点数据
 * x       x轴位置
 * y       y轴位置
 * w       验证点宽度
 * h       验证点高度
 * isIcon  是否是图标
 */
func (c *ClickCaptcha) CheckPosition(pointArr []*Point, x int, y int, w int, h int, isIcon bool) bool {

	flag := true
	for _, v := range pointArr {
		if v.X > 0 && v.Y > 0 {
			flagX := false
			flagY := false
			if (x+w) < v.X || x > v.X+v.Width {
				flagX = true
			}

			currentPhStart := y - h
			currentPhEnd := y
			historyPhStart := v.Y - v.Height
			historyPhEnd := v.Y
			if isIcon {
				currentPhStart = y
				currentPhEnd = y + h
				historyPhStart = v.Y
				historyPhEnd = v.Y + v.Height
			}

			if currentPhEnd < historyPhStart || currentPhStart > historyPhEnd {
				flagY = true
			}
			if !flagX && !flagY {
				flag = false
			}
		}
	}
	return flag
}
