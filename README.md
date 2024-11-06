# 前序准备
 * go 版本1.21.8
 * 安装air
    - ```go install github.com/air-verse/air@latest```
    - 参考https://github.com/cosmtrek/air/blob/master/README-zh_cn.md
 * 安装wire
    - ```go install github.com/google/wire/cmd/wire@latest``` 
    - 参考https://github.com/google/wire
 * 数据库Mysql
 * NodeJs >= 16.14.2
 * Npm >= 8.5.0

# 启动安装服务
   ```
   #其中 go-build-admin 为项目根目录
   cd go-build-admin 

   #  根目录下运行
   air
   ```
   接下来访问 http://127.0.0.1:9989/install,根据引导完成安装即可
# 目录结构
   ```
   Server 端
   ├─app（应用目录）
   │  ├─admin
   │  ├─api
   │  ├─cmd  (配置命令)
   │  ├─common
   │  └─middleware (中间件)
   │
   ├─cmd（main.go 文件）
   │  └─app
   │     ├─app.go  
   │     ├─main.go  (入口文件)
   │     ├─wire_gen.go(wire生成文件无需修改)
   │     └─wire.go
   │
   ├─conf（配置目录）
   │  ├─localize (多语言)
   │  ├─config.local.yaml (本地配置文件)
   │  └─config.yaml
   │
   ├─database（数据库迁移文件）
   │
   ├─router（路由）
   │
   ├─static（静态文件及前端页面）
   │  ├─npm-install-test（npm install 测试项目）
   │  ├─install（安装器源代码，安装后请删除）
   │  ├─fonts（一些字体）
   │  └─images（一些图片）
   │
   │─storage（上传的文件保存在这里）
   │  ├─default（图片）
   │  └─logs（日志）
   │
   ├─tests（测试文件）
   │
   ├─utils（工具库）
   │
   ├─web（WEB端源代码，见下文详叙）
   |
   │  .air.toml（air配置文件）
   │  .gitignore
   │  .go.mod
   │  README.md
   ```

# 特别鸣谢
前端使用的是buildAdmin的前端,做了部分修改,使用请参考https://www.buildadmin.com/


注意:
crud生成代码需修改根目录.air.toml文件delay的时间,这样代码变更后不会马上编译
生成的代码对时间字段处理,需修改前端传的时间格式或者更改字段类型