package captcha

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"go-build-admin/database/migrations/model"
	"go-build-admin/utils"
	"image"
	"image/color"
	"image/draw"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/disintegration/imaging"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"gorm.io/gorm"
)

type CaptchaConfig struct {
	SeKey    string // 验证码加密密钥
	CodeSet  string // 验证码字符集合
	Expire   int    // 验证码过期时间（s）
	UseZh    bool   // 使用中文验证码
	ZhSet    string // 中文验证码字符串
	UseImgBg bool   // 使用背景图片
	FontSize int    // 验证码字体大小(px)
	UseCurve bool   // 是否画混淆曲线
	UseNoise bool   // 是否添加杂点
	ImageH   int    // 验证码图片高度
	ImageW   int    // 验证码图片宽度
	Length   int    // 验证码位数
	FontTTF  string // 验证码字体，不设置随机获取
	Bg       []int  // 背景颜色
	Reset    bool   // 验证成功后是否重置
}

type Captcha struct {
	sqlDB  *gorm.DB
	config CaptchaConfig
}

func NewCaptcha(sqlDB *gorm.DB) *Captcha {
	defaultConfig := CaptchaConfig{
		SeKey:    "BuildAdmin",
		CodeSet:  "2345678abcdefhijkmnpqrstuvwxyzABCDEFGHJKLMNPQRTUVWXY",
		Expire:   600,
		UseZh:    false,
		ZhSet:    "们以我到他会作时要动国产的一是工就年阶义发成部民可出能方进在了不和有大这主中人上为来分生对于学下级地个用同行面说种过命度革而多子后自社加小机也经力线本电高量长党得实家定深法表着水理化争现所二起政三好十战无农使性前等反体合斗路图把结第里正新开论之物从当两些还天资事队批点育重其思与间内去因件日利相由压员气业代全组数果期导平各基或月毛然如应形想制心样干都向变关问比展那它最及外没看治提五解系林者米群头意只明四道马认次文通但条较克又公孔领军流入接席位情运器并飞原油放立题质指建区验活众很教决特此常石强极土少已根共直团统式转别造切九你取西持总料连任志观调七么山程百报更见必真保热委手改管处己将修支识病象几先老光专什六型具示复安带每东增则完风回南广劳轮科北打积车计给节做务被整联步类集号列温装即毫知轴研单色坚据速防史拉世设达尔场织历花受求传口断况采精金界品判参层止边清至万确究书术状厂须离再目海交权且儿青才证低越际八试规斯近注办布门铁需走议县兵固除般引齿千胜细影济白格效置推空配刀叶率述今选养德话查差半敌始片施响收华觉备名红续均药标记难存测士身紧液派准斤角降维板许破述技消底床田势端感往神便贺村构照容非搞亚磨族火段算适讲按值美态黄易彪服早班麦削信排台声该击素张密害侯草何树肥继右属市严径螺检左页抗苏显苦英快称坏移约巴材省黑武培著河帝仅针怎植京助升王眼她抓含苗副杂普谈围食射源例致酸旧却充足短划剂宣环落首尺波承粉践府鱼随考刻靠够满夫失包住促枝局菌杆周护岩师举曲春元超负砂封换太模贫减阳扬江析亩木言球朝医校古呢稻宋听唯输滑站另卫字鼓刚写刘微略范供阿块某功套友限项余倒卷创律雨让骨远帮初皮播优占死毒圈伟季训控激找叫云互跟裂粮粒母练塞钢顶策双留误础吸阻故寸盾晚丝女散焊功株亲院冷彻弹错散商视艺灭版烈零室轻血倍缺厘泵察绝富城冲喷壤简否柱李望盘磁雄似困巩益洲脱投送奴侧润盖挥距触星松送获兴独官混纪依未突架宽冬章湿偏纹吃执阀矿寨责熟稳夺硬价努翻奇甲预职评读背协损棉侵灰虽矛厚罗泥辟告卵箱掌氧恩爱停曾溶营终纲孟钱待尽俄缩沙退陈讨奋械载胞幼哪剥迫旋征槽倒握担仍呀鲜吧卡粗介钻逐弱脚怕盐末阴丰雾冠丙街莱贝辐肠付吉渗瑞惊顿挤秒悬姆烂森糖圣凹陶词迟蚕亿矩康遵牧遭幅园腔订香肉弟屋敏恢忘编印蜂急拿扩伤飞露核缘游振操央伍域甚迅辉异序免纸夜乡久隶缸夹念兰映沟乙吗儒杀汽磷艰晶插埃燃欢铁补咱芽永瓦倾阵碳演威附牙芽永瓦斜灌欧献顺猪洋腐请透司危括脉宜笑若尾束壮暴企菜穗楚汉愈绿拖牛份染既秋遍锻玉夏疗尖殖井费州访吹荣铜沿替滚客召旱悟刺脑措贯藏敢令隙炉壳硫煤迎铸粘探临薄旬善福纵择礼愿伏残雷延烟句纯渐耕跑泽慢栽鲁赤繁境潮横掉锥希池败船假亮谓托伙哲怀割摆贡呈劲财仪沉炼麻罪祖息车穿货销齐鼠抽画饲龙库守筑房歌寒喜哥洗蚀废纳腹乎录镜妇恶脂庄擦险赞钟摇典柄辩竹谷卖乱虚桥奥伯赶垂途额壁网截野遗静谋弄挂课镇妄盛耐援扎虑键归符庆聚绕摩忙舞遇索顾胶羊湖钉仁音迹碎伸灯避泛亡答勇频皇柳哈揭甘诺概宪浓岛袭谁洪谢炮浇斑讯懂灵蛋闭孩释乳巨徒私银伊景坦累匀霉杜乐勒隔弯绩招绍胡呼痛峰零柴簧午跳居尚丁秦稍追梁折耗碱殊岗挖氏刃剧堆赫荷胸衡勤膜篇登驻案刊秧缓凸役剪川雪链渔啦脸户洛孢勃盟买杨宗焦赛旗滤硅炭股坐蒸凝竟陷枪黎救冒暗洞犯筒您宋弧爆谬涂味津臂障褐陆啊健尊豆拔莫抵桑坡缝警挑污冰柬嘴啥饭塑寄赵喊垫丹渡耳刨虎笔稀昆浪萨茶滴浅拥穴覆伦娘吨浸袖珠雌妈紫戏塔锤震岁貌洁剖牢锋疑霸闪埔猛诉刷狠忽灾闹乔唐漏闻沈熔氯荒茎男凡抢像浆旁玻亦忠唱蒙予纷捕锁尤乘乌智淡允叛畜俘摸锈扫毕璃宝芯爷鉴秘净蒋钙肩腾枯抛轨堂拌爸循诱祝励肯酒绳穷塘燥泡袋朗喂铝软渠颗惯贸粪综墙趋彼届墨碍启逆卸航衣孙龄岭骗休借",
		UseImgBg: false,
		FontSize: 25,
		UseCurve: true,
		UseNoise: true,
		ImageH:   0,
		ImageW:   0,
		Length:   4,
		FontTTF:  "",
		Bg:       []int{243, 251, 254},
		Reset:    true,
	}
	return &Captcha{config: defaultConfig, sqlDB: sqlDB}
}

func (c *Captcha) SetConfig(config CaptchaConfig) {
	if config.CodeSet != "" {
		c.config.CodeSet = config.CodeSet
	}

	if config.FontSize != 0 {
		c.config.FontSize = config.FontSize
	}

	if config.Length != 0 {
		c.config.Length = config.Length
	}

	c.config.UseCurve = config.UseCurve
	c.config.CodeSet = config.CodeSet

}

// 验证验证码是否正确
func (c *Captcha) Check(code, id string) bool {
	if code == "" {
		return false
	}
	key := c.authCode(c.config.SeKey, id)
	seCode := model.Captcha{}
	err := c.sqlDB.Model(&model.Captcha{}).Where("`key`=?", key).Scan(&seCode).Error
	if err != nil {
		return false
	}

	if time.Now().Unix() > seCode.ExpireTime {
		c.sqlDB.Model(&model.Captcha{}).Where("`key`=?", key).Delete(nil)
		return false
	}

	if c.authCode(strings.ToUpper(code), id) == seCode.Code {
		if c.config.Reset {
			c.sqlDB.Model(&model.Captcha{}).Where("`key`=?", key).Delete(nil)
		}
		return true
	}
	return false
}

// 创建一个逻辑验证码可供后续验证（非图形）
func (c *Captcha) Create(id string) (string, error) {
	key := c.authCode(c.config.SeKey, id)
	seCode := model.Captcha{}
	err := c.sqlDB.Model(&model.Captcha{}).Where("`key`=?", key).Scan(&seCode).Error
	if err == nil {
		c.sqlDB.Model(&model.Captcha{}).Where("`key`=?", key).Delete(nil)
	}

	captcha := c.generate()
	code := c.authCode(captcha, id)
	// 实现数据库插入操作
	err = c.sqlDB.Model(&model.Captcha{}).Create(map[string]interface{}{
		"key":         key,
		"code":        code,
		"captcha":     captcha,
		"create_time": time.Now().Unix(),
		"expire_time": time.Now().Unix() + int64(c.config.Expire),
	}).Error
	return captcha, err
}

// 获取验证码数据
func (c *Captcha) GetCaptchaData(id string) (model.Captcha, error) {
	key := c.authCode(c.config.SeKey, id)
	seCode := model.Captcha{}
	err := c.sqlDB.Model(&model.Captcha{}).Where("`key`=?", key).Scan(&seCode).Error
	return seCode, err
}

// 输出图形验证码并把验证码的值保存的Mysql中
func (c *Captcha) Entry(id string) (*image.RGBA, error) {
	imageW := c.config.ImageW
	imageH := c.config.ImageH
	if imageW == 0 {
		imageW = int(float64(c.config.Length*c.config.FontSize)*1.5) + c.config.Length*c.config.FontSize/2
	}

	if imageH == 0 {
		imageH = int(float64(c.config.FontSize) * 2.5)
	}
	// 建立一幅
	img := image.NewRGBA(image.Rect(0, 0, imageW, imageH))
	// 设置背景
	backgroundColor := color.RGBA{uint8(c.config.Bg[0]), uint8(c.config.Bg[1]), uint8(c.config.Bg[2]), 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{backgroundColor}, image.Point{}, draw.Src)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// 验证码字体随机颜色
	textColor := color.RGBA{uint8(r.Intn(255)), uint8(r.Intn(255)), uint8(r.Intn(255)), 255}

	if c.config.UseImgBg {
		bgPath := background()
		bgImg, err := loadImage(filepath.Join(utils.RootPath(), bgPath))
		if err != nil {
			return nil, err
		}
		draw.Draw(img, bgImg.Bounds(), bgImg, image.Point{}, draw.Src)
	}

	if c.config.UseNoise {
		writeNoise(img)
	}

	if c.config.UseCurve {
		writeCurve(img, c.config.FontSize, c.config.ImageW, c.config.ImageH, textColor)
	}

	key := c.authCode(c.config.SeKey, id)
	seCode := model.Captcha{}
	err := c.sqlDB.Model(&model.Captcha{}).Where("`key`=?", key).Scan(&seCode).Error

	// 绘验证码
	if err == nil && time.Now().Unix() <= seCode.ExpireTime {
		if _, err = writeText(img, c.config, seCode.Captcha, textColor); err != nil {
			return nil, err
		}
	} else {
		captcha, err := writeText(img, c.config, "", textColor)
		code := c.authCode(captcha, id)

		if err := c.sqlDB.Model(&model.Captcha{}).Where("`key`=?", key).Create(&model.Captcha{
			Key:        key,
			Code:       code,
			Captcha:    captcha,
			CreateTime: time.Now().Unix(),
			ExpireTime: time.Now().Unix() + int64(c.config.Expire),
		}).Error; err != nil {
			return nil, err
		}

		if err != nil {
			return nil, err
		}
	}
	return img, nil
}

// 绘验证码
func writeText(img *image.RGBA, config CaptchaConfig, captcha string, textColor color.RGBA) (string, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	code := []string{}
	runes := []rune{}
	if captcha != "" {
		runes = []rune(captcha)
	} else {
		if config.UseZh {
			runes = []rune(config.ZhSet)
		} else {
			runes = []rune(config.CodeSet)
		}
	}

	// 验证码使用随机字体
	fontTtf := config.FontTTF
	if fontTtf == "" {
		if config.UseZh {
			name := strconv.Itoa(r.Intn(2) + 1)
			fontTtf = filepath.Join(utils.RootPath(), "/static/fonts/zhttfs", name+".ttf")
		} else {
			name := strconv.Itoa(r.Intn(6) + 1)
			fontTtf = filepath.Join(utils.RootPath(), "/static/fonts/ttfs", name+".ttf")
		}
	}

	fontBytes, err := os.ReadFile(fontTtf)
	if err != nil {
		return "", err
	}
	fontData, err := truetype.Parse(fontBytes)
	if err != nil {
		return "", err
	}
	fontFace := truetype.NewFace(fontData, &truetype.Options{Size: float64(config.FontSize)})

	for i := 0; i < config.Length; i++ {
		randomIndex := r.Intn(len(runes))
		code = append(code, string(runes[randomIndex]))
		runes = append(runes[:randomIndex], runes[randomIndex+1:]...)

		textAngle := float64(r.Intn(30)+1) - 15

		pt := image.Point{int(float64(config.FontSize) * (float64(i) + 1) * 1.3), int(config.FontSize + 15)}
		drawRotatedText(img, code[i], pt, textAngle, fontFace, textColor)
	}
	return strings.ToUpper(strings.Join(code, "")), nil
}

// 画一条由两条连在一起构成的随机正弦函数曲线作干扰线
func writeCurve(img *image.RGBA, fontSize int, imageW int, imageH int, textColor color.RGBA) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	py := 0

	// 曲线部分
	A := r.Intn(int(imageH/2)) + 1          // 振幅
	b := r.Intn(int(imageH/2)+1) - imageH/4 // Y轴方向偏移量
	f := r.Intn(int(imageH/2)+1) - imageH/4 // X轴方向偏移量
	T := r.Intn(imageW*2-imageH+1) + imageH // 周期
	w := 2 * math.Pi / float64(T)           // 角频率

	px := 0                                                       // 曲线横坐标起始位置
	px2 := r.Intn(int(float64(imageW)*0.8)-imageW/2+1) + imageW/2 // 曲线横坐标结束位置

	for px < px2 {
		if w != 0 {
			py = int(float64(A)*math.Sin(w*float64(px)+float64(f)) + float64(b) + float64(imageH)/2)
			i := int(fontSize / 5)
			for i >= 0 {
				// 画像素点
				img.Set(px+i, py+i, textColor)
				i--
			}
		}
		px = px + 1
	}

	// 曲线后部分
	A = r.Intn(int(imageH/2)) + 1          // 振幅
	f = r.Intn(int(imageH/2)+1) - imageH/4 // X轴方向偏移量
	T = r.Intn(imageW*2-imageH+1) + imageH // 周期
	w = (2 * math.Pi) / float64(T)
	b = py - A*int(math.Sin(w*float64(px+f))) - int(imageH/2) // Y轴方向偏移量
	px = px2
	px2 = imageW

	for px < px2 {
		if w != 0 {
			py = int(float64(A)*math.Sin(w*float64(px)+float64(f)) + float64(b) + float64(imageH)/2)
			i := int(fontSize / 5)
			for i >= 0 {
				// 画像素点
				img.Set(px+i, py+i, textColor)
				i--
			}
		}
		px = px + 1
	}
}

// 绘杂点，往图片上写不同颜色的字母或数字
func writeNoise(img *image.RGBA) {
	codeSet := "2345678abcdefhijkmnpqrstuvwxyz"
	fontFace := basicfont.Face7x13

	// 设置随机数种子
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 10; i++ {
		// 杂点颜色
		noiseColor := color.RGBA{
			R: uint8(r.Intn(76) + 150),
			G: uint8(r.Intn(76) + 150),
			B: uint8(r.Intn(76) + 150),
			A: 255,
		}

		for j := 0; j < 5; j++ {
			textPoint := image.Pt(r.Intn(img.Bounds().Dx()), r.Intn(img.Bounds().Dy()))
			textAngle := float64(r.Intn(90) + 1) // 45度角

			charIndex := rand.Intn(len(codeSet))
			char := codeSet[charIndex]
			// 绘制旋转后的文本水印
			drawRotatedText(img, string(char), textPoint, textAngle, fontFace, noiseColor)
		}
	}
}

func drawRotatedText(img *image.RGBA, text string, point image.Point, angle float64, fontFace font.Face, textColor color.Color) {
	// 创建一个临时的图像用于绘制旋转后的文本
	textImg := image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))

	// 创建一个基于原图像的绘图上下文
	d := &font.Drawer{
		Dst:  textImg,
		Src:  image.NewUniform(textColor),
		Face: fontFace,
		Dot:  fixed.P(point.X, point.Y),
	}
	d.DrawString(text)
	rotated := imaging.Rotate(textImg, angle, color.NRGBA{0, 0, 0, 0})
	// 将临时图像的内容合并到原始图像中
	draw.Draw(img, img.Bounds(), rotated, image.Point{}, draw.Over)
}

// 背景图片路径
func background() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	bgs := []string{}
	for i := 1; i <= 8; i++ {
		bgs = append(bgs, filepath.Join("static/images/captcha/image/", strconv.Itoa(i)+".jpg"))
	}
	randIndex := r.Intn(len(bgs) - 1)
	imagePath := bgs[randIndex]
	return imagePath
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

// 加密验证码
func (c *Captcha) authCode(str, id string) string {
	key := fmt.Sprintf("%x", md5.Sum([]byte(c.config.SeKey)))[5:13]
	strHash := fmt.Sprintf("%x", md5.Sum([]byte(str)))[8:18]
	return fmt.Sprintf("%x", md5.Sum([]byte(key+strHash+id)))
}

// 生成验证码随机字符
func (c *Captcha) generate() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	code := []string{}
	if c.config.UseZh {
		zhSet := []rune(c.config.ZhSet)
		for i := 0; i < c.config.Length; i++ {
			randIndex := r.Intn(utf8.RuneCountInString(c.config.ZhSet))
			code = append(code, string(zhSet[randIndex]))
		}
	} else {
		for i := 0; i < c.config.Length; i++ {
			randIndex := r.Intn(len(c.config.CodeSet))
			code = append(code, string(c.config.CodeSet[randIndex]))
		}
	}
	return strings.ToUpper(strings.Join(code, ""))
}
