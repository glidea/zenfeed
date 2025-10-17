# Web 项目目录

这个目录包含基于HTML5和JavaScript的网页应用程序。

## 项目列表

### 🎮 Tank Battle (坦克大战)
位置: `tank-game/`

一个完整的网页版坦克大战游戏，使用HTML5 Canvas和原生JavaScript开发。

**特性:**
- 玩家坦克控制系统
- 智能敌方AI
- 子弹系统和碰撞检测
- 可破坏的墙体和障碍物
- 基地保护目标
- 得分系统和关卡系统
- 响应式设计

**如何运行:**
```bash
cd tank-game
python3 -m http.server 8000
# 然后访问 http://localhost:8000
```

详细文档请查看 [tank-game/README.md](tank-game/README.md)

## 开发说明

这些网页应用独立于主项目的Go后端，可以单独运行和部署。

## 许可证

与主项目保持一致，遵循 AGPLv3 许可证。
