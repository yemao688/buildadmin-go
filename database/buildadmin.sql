/*
Navicat MySQL Data Transfer

Source Server         : localhost_3306
Source Server Version : 50726
Source Host           : localhost:3306
Source Database       : buildadmin

Target Server Type    : MYSQL
Target Server Version : 50726
File Encoding         : 65001

Date: 2024-11-05 18:15:31
*/

SET FOREIGN_KEY_CHECKS=0;

-- ----------------------------
-- Table structure for `ba_admin`
-- ----------------------------
DROP TABLE IF EXISTS `ba_admin`;
CREATE TABLE `ba_admin` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `username` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '用户名',
  `nickname` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '昵称',
  `avatar` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '头像',
  `email` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '邮箱',
  `mobile` varchar(11) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '手机',
  `login_failure` tinyint(4) unsigned NOT NULL DEFAULT '0' COMMENT '登录失败次数',
  `last_login_time` bigint(16) unsigned DEFAULT NULL COMMENT '上次登录时间',
  `last_login_ip` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '上次登录IP',
  `password` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '密码',
  `salt` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '密码盐',
  `motto` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '签名',
  `status` enum('0','1') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '1' COMMENT '状态:0=禁用,1=启用',
  `update_time` bigint(16) unsigned DEFAULT NULL COMMENT '更新时间',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `username` (`username`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='管理员表';

-- ----------------------------
-- Records of ba_admin
-- ----------------------------
INSERT INTO `ba_admin` VALUES ('1', 'admin', 'Admin', '/storage/default/20240928/微信截图_20dacc376d191198375cd59f5e3abcd62d58a527de.png', 'admin@buildadmin.com', '18888880000', '0', '1730266693', '127.0.0.1', 'e3c0e9af0e7c595013c922aa5da9bbd1', 'F8YeaAsmZRDOEQd9', '测试签名test', '1', '1730266693', '1715912035');

-- ----------------------------
-- Table structure for `ba_admin_group`
-- ----------------------------
DROP TABLE IF EXISTS `ba_admin_group`;
CREATE TABLE `ba_admin_group` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `pid` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '上级分组',
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '组名',
  `rules` text COLLATE utf8mb4_unicode_ci COMMENT '权限规则ID',
  `status` enum('0','1') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '1' COMMENT '状态:0=禁用,1=启用',
  `update_time` bigint(16) unsigned DEFAULT NULL COMMENT '更新时间',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='管理分组表';

-- ----------------------------
-- Records of ba_admin_group
-- ----------------------------
INSERT INTO `ba_admin_group` VALUES ('1', '0', '超级管理组', '*', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_group` VALUES ('2', '1', '一级管理员', '1,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,77,48,49,50,51,52,53,54,55,56,57,58,59,60,61,62,63,64,65,66,67,68,69,70,71,72,73,74,75,76,89', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_group` VALUES ('3', '2', '二级管理员', '21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_group` VALUES ('4', '3', '三级管理员', '55,56,57,58,59,60,61,62,63,64,65,66,67,68,69,70,71,72,73,74,75', '1', '1715912035', '1715912035');

-- ----------------------------
-- Table structure for `ba_admin_group_access`
-- ----------------------------
DROP TABLE IF EXISTS `ba_admin_group_access`;
CREATE TABLE `ba_admin_group_access` (
  `uid` int(11) unsigned NOT NULL COMMENT '管理员ID',
  `group_id` int(11) unsigned NOT NULL COMMENT '分组ID',
  KEY `uid` (`uid`),
  KEY `group_id` (`group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='管理分组映射表';

-- ----------------------------
-- Records of ba_admin_group_access
-- ----------------------------
INSERT INTO `ba_admin_group_access` VALUES ('1', '1');

-- ----------------------------
-- Table structure for `ba_admin_log`
-- ----------------------------
DROP TABLE IF EXISTS `ba_admin_log`;
CREATE TABLE `ba_admin_log` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `admin_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '管理员ID',
  `username` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '管理员用户名',
  `url` varchar(1500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '操作Url',
  `title` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '日志标题',
  `data` longtext COLLATE utf8mb4_unicode_ci COMMENT '请求数据',
  `ip` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'IP',
  `useragent` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'User-Agent',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='管理员日志表';

-- ----------------------------
-- Records of ba_admin_log
-- ----------------------------

-- ----------------------------
-- Table structure for `ba_admin_rule`
-- ----------------------------
DROP TABLE IF EXISTS `ba_admin_rule`;
CREATE TABLE `ba_admin_rule` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `pid` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '上级菜单',
  `type` enum('menu_dir','menu','button') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'menu' COMMENT '类型:menu_dir=菜单目录,menu=菜单项,button=页面按钮',
  `title` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '标题',
  `name` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '规则名称',
  `path` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '路由路径',
  `icon` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '图标',
  `menu_type` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT '' COMMENT '菜单类型:tab=选项卡,link=链接,iframe=Iframe',
  `url` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'Url',
  `component` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '组件路径',
  `keepalive` tinyint(4) unsigned NOT NULL DEFAULT '0' COMMENT '缓存:0=关闭,1=开启',
  `extend` enum('none','add_rules_only','add_menu_only') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'none' COMMENT '扩展属性:none=无,add_rules_only=只添加为路由,add_menu_only=只添加为菜单',
  `remark` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '备注',
  `weigh` int(11) NOT NULL DEFAULT '0' COMMENT '权重',
  `status` enum('0','1') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '1' COMMENT '状态:0=禁用,1=启用',
  `update_time` bigint(16) unsigned DEFAULT NULL COMMENT '更新时间',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `pid` (`pid`)
) ENGINE=InnoDB AUTO_INCREMENT=90 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='菜单和权限规则表';

-- ----------------------------
-- Records of ba_admin_rule
-- ----------------------------
INSERT INTO `ba_admin_rule` VALUES ('1', '0', 'menu', '控制台', 'dashboard', 'dashboard', 'fa fa-dashboard', 'tab', '', '/src/views/backend/dashboard.vue', '1', 'none', 'Dashboard remark', '999', '1', '1727515213', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('2', '0', 'menu_dir', '权限管理', 'auth', 'auth', 'fa fa-group', null, '', '', '0', 'none', '', '100', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('3', '2', 'menu', '角色组管理', 'auth/group', 'auth/group', 'fa fa-group', 'tab', '', '/src/views/backend/auth/group/index.vue', '1', 'none', 'Role remark', '99', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('4', '3', 'button', '查看', 'auth/group/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('5', '3', 'button', '添加', 'auth/group/add', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('6', '3', 'button', '编辑', 'auth/group/edit', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('7', '3', 'button', '删除', 'auth/group/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('8', '2', 'menu', '管理员管理', 'auth/admin', 'auth/admin', 'el-icon-UserFilled', 'tab', '', '/src/views/backend/auth/admin/index.vue', '1', 'none', '', '98', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('9', '8', 'button', '查看', 'auth/admin/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('10', '8', 'button', '添加', 'auth/admin/add', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('11', '8', 'button', '编辑', 'auth/admin/edit', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('12', '8', 'button', '删除', 'auth/admin/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('13', '2', 'menu', '菜单规则管理', 'auth/rule', 'auth/rule', 'el-icon-Grid', 'tab', '', '/src/views/backend/auth/rule/index.vue', '1', 'none', '', '97', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('14', '13', 'button', '查看', 'auth/rule/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('15', '13', 'button', '添加', 'auth/rule/add', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('16', '13', 'button', '编辑', 'auth/rule/edit', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('17', '13', 'button', '删除', 'auth/rule/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('18', '13', 'button', '快速排序', 'auth/rule/sortable', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('19', '2', 'menu', '管理员日志管理', 'auth/adminLog', 'auth/adminLog', 'el-icon-List', 'tab', '', '/src/views/backend/auth/adminLog/index.vue', '1', 'none', '', '96', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('20', '19', 'button', '查看', 'auth/adminLog/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('21', '0', 'menu_dir', '会员管理', 'user', 'user', 'fa fa-drivers-license', null, '', '', '0', 'none', '', '95', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('22', '21', 'menu', '会员管理', 'user/user', 'user/user', 'fa fa-user', 'tab', '', '/src/views/backend/user/user/index.vue', '1', 'none', '', '94', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('23', '22', 'button', '查看', 'user/user/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('24', '22', 'button', '添加', 'user/user/add', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('25', '22', 'button', '编辑', 'user/user/edit', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('26', '22', 'button', '删除', 'user/user/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('27', '21', 'menu', '会员分组管理', 'user/group', 'user/group', 'fa fa-group', 'tab', '', '/src/views/backend/user/group/index.vue', '1', 'none', '', '93', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('28', '27', 'button', '查看', 'user/group/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('29', '27', 'button', '添加', 'user/group/add', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('30', '27', 'button', '编辑', 'user/group/edit', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('31', '27', 'button', '删除', 'user/group/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('32', '21', 'menu', '会员规则管理', 'user/rule', 'user/rule', 'fa fa-th-list', 'tab', '', '/src/views/backend/user/rule/index.vue', '1', 'none', '', '92', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('33', '32', 'button', '查看', 'user/rule/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('34', '32', 'button', '添加', 'user/rule/add', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('35', '32', 'button', '编辑', 'user/rule/edit', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('36', '32', 'button', '删除', 'user/rule/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('37', '32', 'button', '快速排序', 'user/rule/sortable', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('38', '21', 'menu', '会员余额管理', 'user/moneyLog', 'user/moneyLog', 'el-icon-Money', 'tab', '', '/src/views/backend/user/moneyLog/index.vue', '1', 'none', '', '91', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('39', '38', 'button', '查看', 'user/moneyLog/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('40', '38', 'button', '添加', 'user/moneyLog/add', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('41', '21', 'menu', '会员积分管理', 'user/scoreLog', 'user/scoreLog', 'el-icon-Discount', 'tab', '', '/src/views/backend/user/scoreLog/index.vue', '1', 'none', '', '90', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('42', '41', 'button', '查看', 'user/scoreLog/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('43', '41', 'button', '添加', 'user/scoreLog/add', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('44', '0', 'menu_dir', '常规管理', 'routine', 'routine', 'fa fa-cogs', null, '', '', '0', 'none', '', '89', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('45', '44', 'menu', '系统配置', 'routine/config', 'routine/config', 'el-icon-Tools', 'tab', '', '/src/views/backend/routine/config/index.vue', '1', 'none', '', '88', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('46', '45', 'button', '查看', 'routine/config/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('47', '45', 'button', '编辑', 'routine/config/edit', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('48', '44', 'menu', '附件管理', 'routine/attachment', 'routine/attachment', 'fa fa-folder', 'tab', '', '/src/views/backend/routine/attachment/index.vue', '1', 'none', 'Attachment Remark', '87', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('49', '48', 'button', '查看', 'routine/attachment/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('50', '48', 'button', '编辑', 'routine/attachment/edit', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('51', '48', 'button', '删除', 'routine/attachment/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('52', '44', 'menu', '个人资料', 'routine/adminInfo', 'routine/adminInfo', 'fa fa-user', 'tab', '', '/src/views/backend/routine/adminInfo.vue', '1', 'none', '', '86', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('53', '52', 'button', '查看', 'routine/adminInfo/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('54', '52', 'button', '编辑', 'routine/adminInfo/edit', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('55', '0', 'menu_dir', '数据安全管理', 'security', 'security', 'fa fa-shield', null, '', '', '0', 'none', '', '85', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('56', '55', 'menu', '数据回收站', 'security/dataRecycleLog', 'security/dataRecycleLog', 'fa fa-database', 'tab', '', '/src/views/backend/security/dataRecycleLog/index.vue', '1', 'none', '', '84', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('57', '56', 'button', '查看', 'security/dataRecycleLog/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('58', '56', 'button', '删除', 'security/dataRecycleLog/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('59', '56', 'button', '还原', 'security/dataRecycleLog/restore', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('60', '56', 'button', '查看详情', 'security/dataRecycleLog/info', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('61', '55', 'menu', '敏感数据修改记录', 'security/sensitiveDataLog', 'security/sensitiveDataLog', 'fa fa-expeditedssl', 'tab', '', '/src/views/backend/security/sensitiveDataLog/index.vue', '1', 'none', '', '83', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('62', '61', 'button', '查看', 'security/sensitiveDataLog/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('63', '61', 'button', '删除', 'security/sensitiveDataLog/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('64', '61', 'button', '回滚', 'security/sensitiveDataLog/rollback', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('65', '61', 'button', '查看详情', 'security/sensitiveDataLog/info', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('66', '55', 'menu', '数据回收规则管理', 'security/dataRecycle', 'security/dataRecycle', 'fa fa-database', 'tab', '', '/src/views/backend/security/dataRecycle/index.vue', '1', 'none', 'DataRecycle Remark', '82', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('67', '66', 'button', '查看', 'security/dataRecycle/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('68', '66', 'button', '添加', 'security/dataRecycle/add', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('69', '66', 'button', '编辑', 'security/dataRecycle/edit', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('70', '66', 'button', '删除', 'security/dataRecycle/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('71', '55', 'menu', '敏感字段规则管理', 'security/sensitiveData', 'security/sensitiveData', 'fa fa-expeditedssl', 'tab', '', '/src/views/backend/security/sensitiveData/index.vue', '1', 'none', 'SensitiveData Remark', '81', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('72', '71', 'button', '查看', 'security/sensitiveData/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('73', '71', 'button', '添加', 'security/sensitiveData/add', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('74', '71', 'button', '编辑', 'security/sensitiveData/edit', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('75', '71', 'button', '删除', 'security/sensitiveData/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('76', '0', 'menu', 'BuildAdmin', 'buildadmin', 'buildadmin', 'local-logo', 'link', 'https://doc.buildadmin.com', '', '0', 'none', '', '20', '0', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('77', '45', 'button', '添加', 'routine/config/add', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('78', '0', 'menu', '模块市场', 'moduleStore/moduleStore', 'moduleStore', 'el-icon-GoodsFilled', 'tab', '', '/src/views/backend/module/index.vue', '1', 'none', '', '86', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('79', '78', 'button', '查看', 'moduleStore/moduleStore/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('80', '78', 'button', '安装', 'moduleStore/moduleStore/install', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('81', '78', 'button', '调整状态', 'moduleStore/moduleStore/changeState', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('82', '78', 'button', '卸载', 'moduleStore/moduleStore/uninstall', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('83', '78', 'button', '更新', 'moduleStore/moduleStore/update', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('84', '0', 'menu', 'CRUD代码生成', 'crud/crud', 'crud/crud', 'fa fa-code', 'tab', '', '/src/views/backend/crud/index.vue', '1', 'none', '', '80', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('85', '84', 'button', '查看', 'crud/crud/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('86', '84', 'button', '生成', 'crud/crud/generate', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('87', '84', 'button', '删除', 'crud/crud/delete', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('88', '45', 'button', '删除', 'routine/config/del', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912035', '1715912035');
INSERT INTO `ba_admin_rule` VALUES ('89', '1', 'button', '查看', 'dashboard/index', '', '', null, '', '', '0', 'none', '', '0', '1', '1715912036', '1715912036');

-- ----------------------------
-- Table structure for `ba_area`
-- ----------------------------
DROP TABLE IF EXISTS `ba_area`;
CREATE TABLE `ba_area` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `pid` int(11) unsigned NOT NULL COMMENT '父id',
  `shortname` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '简称',
  `name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '名称',
  `mergename` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '全称',
  `level` tinyint(4) unsigned DEFAULT NULL COMMENT '层级:1=省,2=市,3=区/县',
  `pinyin` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '拼音',
  `code` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '长途区号',
  `zip` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '邮编',
  `first` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '首字母',
  `lng` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '经度',
  `lat` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '纬度',
  PRIMARY KEY (`id`),
  KEY `pid` (`pid`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='省份地区表';

-- ----------------------------
-- Records of ba_area
-- ----------------------------
INSERT INTO `ba_area` VALUES ('1', '0', '湖南', '湖南', '湖南', '1', 'hunan', null, null, 'hn', null, null);
INSERT INTO `ba_area` VALUES ('2', '1', '长沙', '长沙', '长沙', '2', 'changsha', null, null, 'cs', null, null);
INSERT INTO `ba_area` VALUES ('3', '2', '天心区', '天心区', '天心区', '3', 'tianxinqu', null, null, 'txq', null, null);
INSERT INTO `ba_area` VALUES ('4', '2', '芙蓉区', '芙蓉区', '芙蓉区', '3', 'furongqu', null, null, 'fr', null, null);

-- ----------------------------
-- Table structure for `ba_attachment`
-- ----------------------------
DROP TABLE IF EXISTS `ba_attachment`;
CREATE TABLE `ba_attachment` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `topic` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '细目',
  `admin_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '上传管理员ID',
  `user_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '上传用户ID',
  `url` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '物理路径',
  `width` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '宽度',
  `height` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '高度',
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '原始名称',
  `size` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '大小',
  `mimetype` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'mime类型',
  `quote` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '上传(引用)次数',
  `storage` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '存储方式',
  `sha1` varchar(40) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'sha1编码',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  `last_upload_time` bigint(16) unsigned DEFAULT NULL COMMENT '最后上传时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=21 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='附件表';

-- ----------------------------
-- Records of ba_attachment
-- ----------------------------
INSERT INTO `ba_attachment` VALUES ('3', 'default', '1', '0', '/storage/default/20240528/新建文本文档a94a8fe5ccb19ba61c4c0873d391e987982fbbd3.txt', '0', '0', '新建文本文档.txt', '4', 'text/plain', '1', 'local', 'a94a8fe5ccb19ba61c4c0873d391e987982fbbd3', '1716873040', '1716873040');
INSERT INTO `ba_attachment` VALUES ('13', 'default', '0', '18', '/storage/default/20240824/微信截图_20ac6153ae2743afd01b0511f3b5c3b39ada7bf648.png', '238', '104', '微信截图_20ac6153ae2743afd01b0511f3b5c3b39ada7bf648.png', '25003', 'image/png', '1', 'local', 'ac6153ae2743afd01b0511f3b5c3b39ada7bf648', '1724479893', '1724479893');
INSERT INTO `ba_attachment` VALUES ('19', 'default', '1', '0', '/storage/default/20240928/微信图片_20587297ab9faea910d2107b07578461b13c448e7b.jpg', '353', '353', '微信图片_20587297ab9faea910d2107b07578461b13c448e7b.jpg', '23405', 'image/jpeg', '1', 'local', '587297ab9faea910d2107b07578461b13c448e7b', '1727518028', '1727518028');
INSERT INTO `ba_attachment` VALUES ('20', 'default', '1', '0', '/storage/default/20240928/微信截图_20dacc376d191198375cd59f5e3abcd62d58a527de.png', '238', '104', '微信截图_20220424114327.png', '23087', 'image/png', '1', 'local', 'dacc376d191198375cd59f5e3abcd62d58a527de', '1727518301', '1727518301');

-- ----------------------------
-- Table structure for `ba_captcha`
-- ----------------------------
DROP TABLE IF EXISTS `ba_captcha`;
CREATE TABLE `ba_captcha` (
  `key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '验证码Key',
  `code` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '验证码(加密后)',
  `captcha` text COLLATE utf8mb4_unicode_ci COMMENT '验证码数据',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  `expire_time` bigint(16) unsigned DEFAULT NULL COMMENT '过期时间',
  PRIMARY KEY (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='验证码表';

-- ----------------------------
-- Records of ba_captcha
-- ----------------------------

-- ----------------------------
-- Table structure for `ba_config`
-- ----------------------------
DROP TABLE IF EXISTS `ba_config`;
CREATE TABLE `ba_config` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `name` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '变量名',
  `group` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '分组',
  `title` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '变量标题',
  `tip` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '变量描述',
  `type` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '变量输入组件类型',
  `value` longtext COLLATE utf8mb4_unicode_ci COMMENT '变量值',
  `content` longtext COLLATE utf8mb4_unicode_ci COMMENT '字典数据',
  `rule` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '验证规则',
  `extend` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '扩展属性',
  `allow_del` tinyint(4) unsigned NOT NULL DEFAULT '0' COMMENT '允许删除:0=否,1=是',
  `weigh` int(11) NOT NULL DEFAULT '0' COMMENT '权重',
  PRIMARY KEY (`id`),
  UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB AUTO_INCREMENT=51 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='系统配置';

-- ----------------------------
-- Records of ba_config
-- ----------------------------
INSERT INTO `ba_config` VALUES ('1', 'config_group', 'basics', 'Config group', '', 'array', '[{\"key\":\"basics\",\"value\":\"Basics\"},{\"key\":\"mail\",\"value\":\"Mail\"},{\"key\":\"config_quick_entrance\",\"value\":\"Config Quick entrance\"},{\"key\":\"test_data\",\"value\":\"TestData\"}]', null, 'required', '', '0', '-1');
INSERT INTO `ba_config` VALUES ('2', 'site_name', 'basics', 'Site Name', '站点名称', 'string', 'BuildAdmin', null, 'required', '', '0', '99');
INSERT INTO `ba_config` VALUES ('3', 'record_number', 'basics', 'Record number', '域名备案号', 'string', '-', null, '', '', '0', '0');
INSERT INTO `ba_config` VALUES ('4', 'version', 'basics', 'Version number', '系统版本号', 'string', 'v1.0.0', null, 'required', '', '0', '0');
INSERT INTO `ba_config` VALUES ('5', 'time_zone', 'basics', 'time zone', '', 'string', 'Asia/Shanghai', null, 'required', '', '0', '0');
INSERT INTO `ba_config` VALUES ('6', 'no_access_ip', 'basics', 'No access ip', '禁止访问站点的ip列表,一行一个', 'textarea', '', null, '', '', '0', '0');
INSERT INTO `ba_config` VALUES ('7', 'smtp_server', 'mail', 'smtp server', '', 'string', 'smtp.qq.com', null, '', '', '0', '9');
INSERT INTO `ba_config` VALUES ('8', 'smtp_port', 'mail', 'smtp port', '', 'string', '465', null, '', '', '0', '8');
INSERT INTO `ba_config` VALUES ('9', 'smtp_user', 'mail', 'smtp user', '', 'string', '', null, '', '', '0', '7');
INSERT INTO `ba_config` VALUES ('10', 'smtp_pass', 'mail', 'smtp pass', '', 'string', '', null, '', '', '0', '6');
INSERT INTO `ba_config` VALUES ('11', 'smtp_verification', 'mail', 'smtp verification', '', 'select', 'SSL', '{\"SSL\":\"SSL\",\"TLS\":\"TLS\"}', '', '', '0', '5');
INSERT INTO `ba_config` VALUES ('12', 'smtp_sender_mail', 'mail', 'smtp sender mail', '', 'string', '11@qq.com', null, 'email', '', '0', '4');
INSERT INTO `ba_config` VALUES ('13', 'config_quick_entrance', 'config_quick_entrance', 'Config Quick entrance', '', 'array', '[{\"key\":\"数据回收规则配置\",\"value\":\"/admin/security/dataRecycle\"},{\"key\":\"敏感数据规则配置\",\"value\":\"/admin/security/sensitiveData\"}]', null, '', '', '0', '0');
INSERT INTO `ba_config` VALUES ('50', 'book', 'test_data', '书', '请输入书名', 'string', '测试书名', '', 'required', '', '0', '2');

-- ----------------------------
-- Table structure for `ba_crud_log`
-- ----------------------------
DROP TABLE IF EXISTS `ba_crud_log`;
CREATE TABLE `ba_crud_log` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `table_name` varchar(200) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '数据表名',
  `table` text COLLATE utf8mb4_unicode_ci COMMENT '数据表数据',
  `fields` text COLLATE utf8mb4_unicode_ci COMMENT '字段数据',
  `status` enum('delete','success','error','start') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'start' COMMENT '状态:delete=已删除,success=成功,error=失败,start=生成中',
  `create_time` bigint(20) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='CRUD记录表';

-- ----------------------------
-- Records of ba_crud_log
-- ----------------------------

-- ----------------------------
-- Table structure for `ba_migrations`
-- ----------------------------
DROP TABLE IF EXISTS `ba_migrations`;
CREATE TABLE `ba_migrations` (
  `version` bigint(20) NOT NULL,
  `migration_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `start_time` timestamp NULL DEFAULT NULL,
  `end_time` timestamp NULL DEFAULT NULL,
  `breakpoint` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ----------------------------
-- Records of ba_migrations
-- ----------------------------
INSERT INTO `ba_migrations` VALUES ('20230620180908', 'Install', '2024-05-17 10:13:55', '2024-05-17 10:13:55', '0');
INSERT INTO `ba_migrations` VALUES ('20230620180916', 'InstallData', '2024-05-17 10:13:55', '2024-05-17 10:13:55', '0');
INSERT INTO `ba_migrations` VALUES ('20230622221507', 'Version200', '2024-05-17 10:13:55', '2024-05-17 10:13:56', '0');
INSERT INTO `ba_migrations` VALUES ('20230719211338', 'Version201', '2024-05-17 10:13:56', '2024-05-17 10:13:56', '0');
INSERT INTO `ba_migrations` VALUES ('20230905060702', 'Version202', '2024-05-17 10:13:56', '2024-05-17 10:13:56', '0');

-- ----------------------------
-- Table structure for `ba_security_data_recycle`
-- ----------------------------
DROP TABLE IF EXISTS `ba_security_data_recycle`;
CREATE TABLE `ba_security_data_recycle` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `name` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '规则名称',
  `controller` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '控制器',
  `controller_as` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '控制器别名',
  `data_table` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '对应数据表',
  `primary_key` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '数据表主键',
  `status` enum('0','1') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '1' COMMENT '状态:0=禁用,1=启用',
  `update_time` bigint(16) unsigned DEFAULT NULL COMMENT '更新时间',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='回收规则表';

-- ----------------------------
-- Records of ba_security_data_recycle
-- ----------------------------
INSERT INTO `ba_security_data_recycle` VALUES ('1', '管理员', 'auth/Admin.php', 'auth/admin', 'admin', 'id', '1', '1715912035', '1715912035');
INSERT INTO `ba_security_data_recycle` VALUES ('2', '管理员日志', 'auth/AdminLog.php', 'auth/adminlog', 'admin_log', 'id', '1', '1715912035', '1715912035');
INSERT INTO `ba_security_data_recycle` VALUES ('3', '菜单规则', 'auth/Menu.php', 'auth/menu', 'menu_rule', 'id', '1', '1715912035', '1715912035');
INSERT INTO `ba_security_data_recycle` VALUES ('4', '系统配置项', 'routine/Config.php', 'routine/config', 'config', 'id', '1', '1715912035', '1715912035');
INSERT INTO `ba_security_data_recycle` VALUES ('5', '会员', 'user/User.php', 'user/user', 'user', 'id', '1', '1715912035', '1715912035');
INSERT INTO `ba_security_data_recycle` VALUES ('6', '数据回收规则', 'security/DataRecycle.php', 'security/datarecycle', 'security_data_recycle', 'id', '1', '1715912035', '1715912035');
INSERT INTO `ba_security_data_recycle` VALUES ('7', '会员', 'admin/user.User', 'admin/user.User', 'user', 'id', '1', '1729048249', '1729048249');

-- ----------------------------
-- Table structure for `ba_security_data_recycle_log`
-- ----------------------------
DROP TABLE IF EXISTS `ba_security_data_recycle_log`;
CREATE TABLE `ba_security_data_recycle_log` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `admin_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '操作管理员',
  `recycle_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '回收规则ID',
  `data` text COLLATE utf8mb4_unicode_ci COMMENT '回收的数据',
  `data_table` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '数据表',
  `primary_key` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '数据表主键',
  `is_restore` tinyint(4) unsigned NOT NULL DEFAULT '0' COMMENT '是否已还原:0=否,1=是',
  `ip` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '操作者IP',
  `useragent` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'User-Agent',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=13 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='数据回收记录表';

-- ----------------------------
-- Records of ba_security_data_recycle_log
-- ----------------------------
INSERT INTO `ba_security_data_recycle_log` VALUES ('2', '1', '5', '{\"id\":5,\"group_id\":0,\"username\":\"test1\",\"nickname\":\"test1\",\"email\":\"\",\"mobile\":\"\",\"avatar\":\"\",\"gender\":0,\"birthday\":null,\"money\":0,\"score\":0,\"last_login_time\":null,\"last_login_ip\":\"\",\"login_failure\":0,\"join_ip\":\"\",\"join_time\":null,\"motto\":\"\",\"password\":\"30ae7e1b1f7d4ce67a36af6f59449073\",\"salt\":\"YJiQUvOE2V9mRPsf\",\"status\":\"enable\",\"update_time\":1716531649,\"create_time\":1716531649}', 'user', 'id', '0', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36', '1716531654');
INSERT INTO `ba_security_data_recycle_log` VALUES ('4', '1', '5', '{\"id\":20,\"group_id\":1,\"username\":\"test89\",\"nickname\":\"test891\",\"email\":\"\",\"mobile\":\"\",\"avatar\":\"\\/static\\/images\\/avatar.png\",\"gender\":0,\"birthday\":\"2024-08-20\",\"money\":20000,\"score\":2,\"last_login_time\":null,\"last_login_ip\":\"\",\"login_failure\":0,\"join_ip\":\"\",\"join_time\":0,\"motto\":\"test\",\"password\":\"78625125ddb4fa7975f46223c3297f39\",\"salt\":\"cXijyrsWTbIlLUfn\",\"status\":\"enable\",\"update_time\":1724482535,\"create_time\":1724482535}', 'user', 'id', '0', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '1727600349');
INSERT INTO `ba_security_data_recycle_log` VALUES ('7', '1', '7', '{\"avatar\":\"/storage/default/20240824/微信图片_20587297ab9faea910d2107b07578461b13c448e7b.jpg\",\"birthday\":\"2024-01-01T00:00:00+08:00\",\"create_time\":1716533281,\"email\":\"\",\"gender\":1,\"group_id\":1,\"id\":18,\"join_ip\":\"\",\"join_time\":0,\"last_login_ip\":\"::1\",\"last_login_time\":1725005764,\"login_failure\":0,\"mobile\":\"\",\"money\":7400,\"motto\":\"test6\",\"nickname\":\"test6\",\"password\":\"a3d737e6c7cbdd2dc50370597e3850bc\",\"salt\":\"lLASuJzhVHT9Wnfk\",\"score\":209,\"status\":\"enable\",\"update_time\":1724913997,\"username\":\"test6\"}', 'user', 'id', '0', '::1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '1729048952');
INSERT INTO `ba_security_data_recycle_log` VALUES ('8', '1', '5', '{\"id\":17,\"group_id\":1,\"username\":\"test1\",\"nickname\":\"test1\",\"email\":\"\",\"mobile\":\"\",\"avatar\":\"\\/static\\/images\\/avatar.png\",\"gender\":1,\"birthday\":\"0000-00-00\",\"money\":0,\"score\":0,\"last_login_time\":null,\"last_login_ip\":\"\",\"login_failure\":0,\"join_ip\":\"\",\"join_time\":0,\"motto\":\"\",\"password\":\"6b62f06fe97d2b08adb795551cee2388\",\"salt\":\"sd0xEnW3FHy9vQbG\",\"status\":\"disable\",\"update_time\":1716533209,\"create_time\":1716533209}', 'user', 'id', '0', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '1729048965');
INSERT INTO `ba_security_data_recycle_log` VALUES ('9', '1', '7', '{\"avatar\":\"/storage/default/20240824/微信图片_20587297ab9faea910d2107b07578461b13c448e7b.jpg\",\"birthday\":\"2024-01-01T00:00:00+08:00\",\"create_time\":1716533281,\"email\":\"\",\"gender\":1,\"group_id\":1,\"id\":18,\"join_ip\":\"\",\"join_time\":0,\"last_login_ip\":\"::1\",\"last_login_time\":1725005764,\"login_failure\":0,\"mobile\":\"\",\"money\":7400,\"motto\":\"test6\",\"nickname\":\"test6\",\"password\":\"a3d737e6c7cbdd2dc50370597e3850bc\",\"salt\":\"lLASuJzhVHT9Wnfk\",\"score\":209,\"status\":\"enable\",\"update_time\":1724913997,\"username\":\"test6\"}', 'user', 'id', '0', '::1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '1729058681');
INSERT INTO `ba_security_data_recycle_log` VALUES ('10', '1', '7', '{\"avatar\":\"/static/images/avatar.png\",\"birthday\":\"2024-05-21T00:00:00+08:00\",\"create_time\":1716259940,\"email\":\"11222@qq.com\",\"gender\":1,\"group_id\":1,\"id\":3,\"join_ip\":\"\",\"join_time\":0,\"last_login_ip\":\"\",\"last_login_time\":null,\"login_failure\":0,\"mobile\":\"15111200555\",\"money\":350,\"motto\":\"\",\"nickname\":\"orange\",\"password\":\"29784cc6103012821d2d3002d762cac5\",\"salt\":\"EMJ4Y5I8lZVbt7F3\",\"score\":0,\"status\":\"enable\",\"update_time\":1716458710,\"username\":\"orange\"}', 'user', 'id', '0', '::1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '1730283299');
INSERT INTO `ba_security_data_recycle_log` VALUES ('11', '1', '7', '{\"avatar\":\"/static/images/avatar.png\",\"birthday\":\"2020-01-16T00:00:00+08:00\",\"create_time\":1716531976,\"email\":\"\",\"gender\":1,\"group_id\":1,\"id\":6,\"join_ip\":\"\",\"join_time\":0,\"last_login_ip\":\"::1\",\"last_login_time\":1727493374,\"login_failure\":0,\"mobile\":\"\",\"money\":200,\"motto\":\"test 个性签名\",\"nickname\":\"test59\",\"password\":\"79d8a73220e00d0ca65949e65547cef8\",\"salt\":\"aDzWTYO1XR5mhK4u\",\"score\":314,\"status\":\"enable\",\"update_time\":1727492781,\"username\":\"test59\"}', 'user', 'id', '0', '::1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '1730283299');
INSERT INTO `ba_security_data_recycle_log` VALUES ('12', '1', '7', '{\"avatar\":\"/storage/default/20240824/微信图片_20587297ab9faea910d2107b07578461b13c448e7b.jpg\",\"birthday\":\"2024-01-01T00:00:00+08:00\",\"create_time\":1716533281,\"email\":\"\",\"gender\":1,\"group_id\":1,\"id\":18,\"join_ip\":\"\",\"join_time\":0,\"last_login_ip\":\"::1\",\"last_login_time\":1725005764,\"login_failure\":0,\"mobile\":\"\",\"money\":7400,\"motto\":\"test6\",\"nickname\":\"test6\",\"password\":\"a3d737e6c7cbdd2dc50370597e3850bc\",\"salt\":\"lLASuJzhVHT9Wnfk\",\"score\":209,\"status\":\"enable\",\"update_time\":1724913997,\"username\":\"test6\"}', 'user', 'id', '0', '::1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '1730283299');

-- ----------------------------
-- Table structure for `ba_security_sensitive_data`
-- ----------------------------
DROP TABLE IF EXISTS `ba_security_sensitive_data`;
CREATE TABLE `ba_security_sensitive_data` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `name` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '规则名称',
  `controller` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '控制器',
  `controller_as` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '控制器别名',
  `data_table` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '对应数据表',
  `primary_key` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '数据表主键',
  `data_fields` text COLLATE utf8mb4_unicode_ci COMMENT '敏感数据字段',
  `status` enum('0','1') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '1' COMMENT '状态:0=禁用,1=启用',
  `update_time` bigint(16) unsigned DEFAULT NULL COMMENT '更新时间',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='敏感数据规则表';

-- ----------------------------
-- Records of ba_security_sensitive_data
-- ----------------------------
INSERT INTO `ba_security_sensitive_data` VALUES ('1', '管理员数据', 'auth/Admin.php', 'auth/admin', 'admin', 'id', '{\"username\":\"用户名\",\"mobile\":\"手机\",\"password\":\"密码\",\"status\":\"状态\"}', '1', '1715912035', '1715912035');
INSERT INTO `ba_security_sensitive_data` VALUES ('2', '会员数据', 'user/User.php', 'user/user', 'user', 'id', '{\"username\":\"用户名\",\"mobile\":\"手机号\",\"password\":\"密码\",\"status\":\"状态\",\"email\":\"邮箱地址\"}', '1', '1715912035', '1715912035');
INSERT INTO `ba_security_sensitive_data` VALUES ('3', '管理员权限', 'auth/Group.php', 'auth/group', 'admin_group', 'id', '{\"rules\":\"权限规则ID\"}', '1', '1715912035', '1715912035');
INSERT INTO `ba_security_sensitive_data` VALUES ('4', '管理员修改', 'admin/auth.Admin', 'admin/auth.Admin', 'admin', 'id', '{\"email\":\"邮箱\"}', '1', '1729045493', '1729045339');

-- ----------------------------
-- Table structure for `ba_security_sensitive_data_log`
-- ----------------------------
DROP TABLE IF EXISTS `ba_security_sensitive_data_log`;
CREATE TABLE `ba_security_sensitive_data_log` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `admin_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '操作管理员',
  `sensitive_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '敏感数据规则ID',
  `data_table` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '数据表',
  `primary_key` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '数据表主键',
  `data_field` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '被修改字段',
  `data_comment` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '被修改项',
  `id_value` int(11) NOT NULL DEFAULT '0' COMMENT '被修改项主键值',
  `before` text COLLATE utf8mb4_unicode_ci COMMENT '修改前',
  `after` text COLLATE utf8mb4_unicode_ci COMMENT '修改后',
  `ip` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '操作者IP',
  `useragent` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'User-Agent',
  `is_rollback` tinyint(4) unsigned NOT NULL DEFAULT '0' COMMENT '是否已回滚:0=否,1=是',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=16 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='敏感数据修改记录';

-- ----------------------------
-- Records of ba_security_sensitive_data_log
-- ----------------------------
INSERT INTO `ba_security_sensitive_data_log` VALUES ('1', '1', '2', 'user', 'id', 'password', '密码', '3', '1d747ecf5bc1fee7594a1c2b678bcd74', '******', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36', '0', '1716259951');
INSERT INTO `ba_security_sensitive_data_log` VALUES ('4', '1', '3', 'admin_group', 'id', 'rules', '权限规则ID', '16', '1,89', null, '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36', '0', '1716366545');
INSERT INTO `ba_security_sensitive_data_log` VALUES ('5', '1', '3', 'admin_group', 'id', 'rules', '权限规则ID', '16', '1,89', null, '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36', '0', '1716366553');
INSERT INTO `ba_security_sensitive_data_log` VALUES ('6', '1', '3', 'admin_group', 'id', 'rules', '权限规则ID', '16', '1,89,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20', null, '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36', '0', '1716366574');
INSERT INTO `ba_security_sensitive_data_log` VALUES ('8', '1', '1', 'admin', 'id', 'username', '用户名', '6', 'test11222', 'test112222', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36', '0', '1716448378');
INSERT INTO `ba_security_sensitive_data_log` VALUES ('9', '1', '2', 'user', 'id', 'password', '密码', '4', 'ff16771ea5517934e2a8a1b2a8e6d4b2', '******', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36', '0', '1716531667');
INSERT INTO `ba_security_sensitive_data_log` VALUES ('10', '1', '3', 'admin_group', 'id', 'rules', '权限规则ID', '7', '1,89,4,5,6,2,3', null, '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '0', '1727501863');
INSERT INTO `ba_security_sensitive_data_log` VALUES ('11', '1', '3', 'admin_group', 'id', 'rules', '权限规则ID', '9', '1,89', null, '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '0', '1727502205');
INSERT INTO `ba_security_sensitive_data_log` VALUES ('12', '1', '3', 'admin_group', 'id', 'rules', '权限规则ID', '6', '1,89', null, '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '0', '1727600272');
INSERT INTO `ba_security_sensitive_data_log` VALUES ('13', '1', '1', 'admin', 'id', 'status', '状态', '3', '1', '0', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '0', '1729044538');
INSERT INTO `ba_security_sensitive_data_log` VALUES ('14', '1', '4', 'admin', 'id', 'email', '邮箱', '3', '22255@qq.com', '2225566@qq.com', '::1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '0', '1729047830');
INSERT INTO `ba_security_sensitive_data_log` VALUES ('15', '1', '4', 'admin', 'id', 'email', '邮箱', '3', '22255@qq.com', '111@qq.com', '::1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36', '0', '1729058828');

-- ----------------------------
-- Table structure for `ba_test_build`
-- ----------------------------
DROP TABLE IF EXISTS `ba_test_build`;
CREATE TABLE `ba_test_build` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `title` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '标题',
  `keyword_rows` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '关键词',
  `content` text COLLATE utf8mb4_unicode_ci COMMENT '内容',
  `likess` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '有帮助数',
  `dislikes` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '无帮助数',
  `note_textarea` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '备注',
  `status` enum('0','1') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '1' COMMENT '状态:0=隐藏,1=正常',
  `weigh` int(11) NOT NULL DEFAULT '0' COMMENT '权重',
  `update_time` bigint(20) unsigned DEFAULT NULL COMMENT '更新时间',
  `create_time` bigint(20) unsigned DEFAULT NULL COMMENT '创建时间',
  `views` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '浏览量',
  `likes` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '有帮助数',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='知识库表';

-- ----------------------------
-- Records of ba_test_build
-- ----------------------------

-- ----------------------------
-- Table structure for `ba_token`
-- ----------------------------
DROP TABLE IF EXISTS `ba_token`;
CREATE TABLE `ba_token` (
  `token` varchar(70) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'Token',
  `type` varchar(15) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '类型',
  `user_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '用户ID',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  `expire_time` bigint(16) unsigned DEFAULT NULL COMMENT '过期时间',
  PRIMARY KEY (`token`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='用户Token表';

-- ----------------------------
-- Records of ba_token
-- ----------------------------
INSERT INTO `ba_token` VALUES ('c27ceea50d702cbef84d11fa8369d824186b7be5788670c1d94bc1f7b12a63a3', 'admin', '1', '1730186751', '1730445951');
INSERT INTO `ba_token` VALUES ('ddb9df37d8a59a1b5194c15154f9fd7f61eecd77', 'admin', '1', '1730102091', '1730361291');

-- ----------------------------
-- Table structure for `ba_user`
-- ----------------------------
DROP TABLE IF EXISTS `ba_user`;
CREATE TABLE `ba_user` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `group_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '分组ID',
  `username` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '用户名',
  `nickname` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '昵称',
  `email` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '邮箱',
  `mobile` varchar(11) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '手机',
  `avatar` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '头像',
  `gender` tinyint(4) unsigned NOT NULL DEFAULT '0' COMMENT '性别:0=未知,1=男,2=女',
  `birthday` date DEFAULT NULL COMMENT '生日',
  `money` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '余额',
  `score` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '积分',
  `last_login_time` bigint(16) unsigned DEFAULT NULL COMMENT '上次登录时间',
  `last_login_ip` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '上次登录IP',
  `login_failure` tinyint(4) unsigned NOT NULL DEFAULT '0' COMMENT '登录失败次数',
  `join_ip` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '加入IP',
  `join_time` bigint(16) unsigned DEFAULT NULL COMMENT '加入时间',
  `motto` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '签名',
  `password` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '密码',
  `salt` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '密码盐',
  `status` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '状态',
  `update_time` bigint(16) unsigned DEFAULT NULL COMMENT '更新时间',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `username` (`username`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='会员表';

-- ----------------------------
-- Records of ba_user
-- ----------------------------
INSERT INTO `ba_user` VALUES ('1', '1', 'user', 'User', '18888888888@qq.com', '18888888888', '', '2', '2024-05-17', '12900', '3', null, '', '0', '', null, '', '9b4b2d94de3f6a03785f8985e810d9be', 'yiTNgBkJufH6zFnj', 'enable', '1716458060', '1715912035');

-- ----------------------------
-- Table structure for `ba_user_group`
-- ----------------------------
DROP TABLE IF EXISTS `ba_user_group`;
CREATE TABLE `ba_user_group` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `name` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '组名',
  `rules` text COLLATE utf8mb4_unicode_ci COMMENT '权限节点',
  `status` enum('0','1') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '1' COMMENT '状态:0=禁用,1=启用',
  `update_time` bigint(16) unsigned DEFAULT NULL COMMENT '更新时间',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='会员组表';

-- ----------------------------
-- Records of ba_user_group
-- ----------------------------
INSERT INTO `ba_user_group` VALUES ('1', '默认分组', '*', '1', '1715912035', '1715912035');

-- ----------------------------
-- Table structure for `ba_user_money_log`
-- ----------------------------
DROP TABLE IF EXISTS `ba_user_money_log`;
CREATE TABLE `ba_user_money_log` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `user_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '会员ID',
  `money` int(11) NOT NULL DEFAULT '0' COMMENT '变更余额',
  `before` int(11) NOT NULL DEFAULT '0' COMMENT '变更前余额',
  `after` int(11) NOT NULL DEFAULT '0' COMMENT '变更后余额',
  `memo` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '备注',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='会员余额变动表';

-- ----------------------------
-- Records of ba_user_money_log
-- ----------------------------

-- ----------------------------
-- Table structure for `ba_user_rule`
-- ----------------------------
DROP TABLE IF EXISTS `ba_user_rule`;
CREATE TABLE `ba_user_rule` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `pid` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '上级菜单',
  `type` enum('route','menu_dir','menu','nav_user_menu','nav','button') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'menu' COMMENT '类型:route=路由,menu_dir=菜单目录,menu=菜单项,nav_user_menu=顶栏会员菜单下拉项,nav=顶栏菜单项,button=页面按钮',
  `title` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '标题',
  `name` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '规则名称',
  `path` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '路由路径',
  `icon` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '图标',
  `menu_type` enum('tab','link','iframe') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'tab' COMMENT '菜单类型:tab=选项卡,link=链接,iframe=Iframe',
  `url` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'Url',
  `component` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '组件路径',
  `no_login_valid` tinyint(4) unsigned NOT NULL DEFAULT '0' COMMENT '未登录有效:0=否,1=是',
  `extend` enum('none','add_rules_only','add_menu_only') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'none' COMMENT '扩展属性:none=无,add_rules_only=只添加为路由,add_menu_only=只添加为菜单',
  `remark` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '备注',
  `weigh` int(11) NOT NULL DEFAULT '0' COMMENT '权重',
  `status` enum('0','1') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '1' COMMENT '状态:0=禁用,1=启用',
  `update_time` bigint(16) unsigned DEFAULT NULL COMMENT '更新时间',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `pid` (`pid`)
) ENGINE=InnoDB AUTO_INCREMENT=7 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='会员菜单权限规则表';

-- ----------------------------
-- Records of ba_user_rule
-- ----------------------------
INSERT INTO `ba_user_rule` VALUES ('1', '0', 'menu_dir', '我的账户', 'account', 'account', 'fa fa-user-circle', 'tab', '', '', '0', 'none', '', '98', '1', '1715912035', '1715912035');
INSERT INTO `ba_user_rule` VALUES ('2', '1', 'menu', '账户概览', 'account/overview', 'account/overview', 'fa fa-home', 'tab', '', '/src/views/frontend/user/account/overview.vue', '0', 'none', '', '99', '1', '1715912035', '1715912035');
INSERT INTO `ba_user_rule` VALUES ('3', '1', 'menu', '个人资料', 'account/profile', 'account/profile', 'fa fa-user-circle-o', 'tab', '', '/src/views/frontend/user/account/profile.vue', '0', 'none', '', '98', '1', '1715912035', '1715912035');
INSERT INTO `ba_user_rule` VALUES ('4', '1', 'menu', '修改密码', 'account/changePassword', 'account/changePassword', 'fa fa-shield', 'tab', '', '/src/views/frontend/user/account/changePassword.vue', '0', 'none', '', '97', '1', '1715912035', '1715912035');
INSERT INTO `ba_user_rule` VALUES ('5', '1', 'menu', '积分记录', 'account/integral', 'account/integral', 'fa fa-tag', 'tab', '', '/src/views/frontend/user/account/integral.vue', '0', 'none', '', '96', '1', '1715912035', '1715912035');
INSERT INTO `ba_user_rule` VALUES ('6', '1', 'menu', '余额记录', 'account/balance', 'account/balance', 'fa fa-money', 'tab', '', '/src/views/frontend/user/account/balance.vue', '0', 'none', '', '95', '1', '1715912035', '1715912035');

-- ----------------------------
-- Table structure for `ba_user_score_log`
-- ----------------------------
DROP TABLE IF EXISTS `ba_user_score_log`;
CREATE TABLE `ba_user_score_log` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `user_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '会员ID',
  `score` int(11) NOT NULL DEFAULT '0' COMMENT '变更积分',
  `before` int(11) NOT NULL DEFAULT '0' COMMENT '变更前积分',
  `after` int(11) NOT NULL DEFAULT '0' COMMENT '变更后积分',
  `memo` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '备注',
  `create_time` bigint(16) unsigned DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='会员积分变动表';

-- ----------------------------
-- Records of ba_user_score_log
-- ----------------------------
