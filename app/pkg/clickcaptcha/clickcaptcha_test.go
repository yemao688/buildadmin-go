package clickcaptcha

import (
	"fmt"
	"go-build-admin/utils"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/golang/freetype/truetype"
	"github.com/magiconair/properties/assert"
	"golang.org/x/image/font"
)

func TestRandPosition(t *testing.T) {

	pointArr := []*Point{
		{Size: 21, Icon: true, Name: "bicycle", Text: "<自行车>", Width: 32, Height: 32, X: 0, Y: 0},
		{Size: 18, Icon: true, Name: "wolf head", Text: "<狼头>", Width: 32, Height: 32, X: 0, Y: 0},
		{Size: 27, Icon: true, Name: "bomb", Text: "<炸弹>", Width: 32, Height: 32, X: 0, Y: 0},
		// {Size: 27, Icon: true, Name: "bomb", Text: "<炸弹>", Width: 32, Height: 32, X: 0, Y: 0},
		{Size: 28, Icon: false, Name: "", Text: "三", Width: 32, Height: 32, X: 0, Y: 0},
	}
	clickCaptcha := ClickCaptcha{}
	for _, v := range pointArr {
		v.X, v.Y = clickCaptcha.RandPosition(pointArr, 350, 200, v.Height, v.Height, v.Icon)
	}
	for _, v := range pointArr {
		assert.Equal(t, true, v.X >= 0 && v.X <= 350, "x 位置超出范围")
		assert.Equal(t, true, v.Y >= 0 && v.Y <= 350, "y 位置超出范围")
	}
}

func TestGetFontWidthAndHeight(t *testing.T) {
	fontBytes, err := os.ReadFile(utils.RootPath() + "/static/fonts/zhttfs/SourceHanSansCN-Normal.ttf")
	if err != nil {
		fmt.Println("加载字体失败")
	}

	f, err := truetype.Parse(fontBytes)
	if err != nil {
		fmt.Println("解析字体失败")
	}

	face := truetype.NewFace(f, &truetype.Options{
		Size:    float64(26),
		Hinting: font.HintingFull,
	})
	text := "字a"
	bounds, _ := font.BoundString(face, text)
	textWidth := bounds.Max.X.Ceil() - bounds.Min.X.Floor()
	textHeight := bounds.Max.Y.Ceil() - bounds.Min.Y.Floor()
	fmt.Printf("%v,%v \n", textWidth, textHeight)
}

func TestAlpha(t *testing.T) {
	fmt.Println(127 - 36*float64(127)/100)

	info := ";350;200"
	infoArr := strings.Split(info, ";")
	fmt.Printf("%+v \n", infoArr)
	xyArr := strings.Split(infoArr[0], "-")
	fmt.Printf("%+v \n", xyArr)
	w, _ := strconv.Atoi(infoArr[1])
	h, _ := strconv.Atoi(infoArr[2])
	fmt.Printf("x:%+v ,y:%+v \n", w, h)

	fmt.Printf("len:%+v \n", len(xyArr))
	xy := strings.Split(xyArr[0], ",")
	x, _ := strconv.Atoi(xy[0])
	y, _ := strconv.Atoi(xy[1])
	fmt.Println(x, y)
}
