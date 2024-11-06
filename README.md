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

# 特别鸣谢
前端使用的是buildAdmin的前端,做了部分修改,使用请参考https://www.buildadmin.com/


注意:
crud生成代码需修改根目录.air.toml文件delay的时间,这样代码变更后不会马上编译
生成的代码对时间字段处理,需修改前端传的时间格式或者更改字段类型