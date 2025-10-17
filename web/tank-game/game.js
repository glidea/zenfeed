// ==================== 游戏配置 Game Configuration ====================
const GAME_CONFIG = {
    CANVAS_WIDTH: 800,
    CANVAS_HEIGHT: 600,
    TANK_SIZE: 30,
    BULLET_SIZE: 6,
    BULLET_SPEED: 5,
    PLAYER_SPEED: 3,
    ENEMY_SPEED: 2,
    WALL_SIZE: 20,
    BASE_SIZE: 40,
    FIRE_COOLDOWN: 500, // 毫秒
    ENEMY_FIRE_INTERVAL: 2000, // 敌人射击间隔
    INITIAL_LIVES: 3,
    ENEMIES_PER_LEVEL: 5,
    MAX_ENEMIES_ON_SCREEN: 3
};

// ==================== 方向枚举 Direction Enum ====================
const DIRECTION = {
    UP: 0,
    RIGHT: 1,
    DOWN: 2,
    LEFT: 3
};

// ==================== 坦克类 Tank Class ====================
class Tank {
    constructor(x, y, isPlayer = false) {
        this.x = x;
        this.y = y;
        this.width = GAME_CONFIG.TANK_SIZE;
        this.height = GAME_CONFIG.TANK_SIZE;
        this.direction = DIRECTION.UP;
        this.speed = isPlayer ? GAME_CONFIG.PLAYER_SPEED : GAME_CONFIG.ENEMY_SPEED;
        this.isPlayer = isPlayer;
        this.lastFired = 0;
        this.alive = true;
        
        // AI相关属性
        if (!isPlayer) {
            this.lastDirectionChange = Date.now();
            this.directionChangeCooldown = 2000 + Math.random() * 2000;
            this.lastFiredTime = Date.now();
        }
    }

    draw(ctx) {
        if (!this.alive) return;

        ctx.save();
        ctx.translate(this.x + this.width / 2, this.y + this.height / 2);
        ctx.rotate((this.direction * Math.PI) / 2);
        
        // 坦克主体
        ctx.fillStyle = this.isPlayer ? '#4A90E2' : '#E74C3C';
        ctx.fillRect(-this.width / 2, -this.height / 2, this.width, this.height);
        
        // 坦克轮廓
        ctx.strokeStyle = this.isPlayer ? '#2E5C8A' : '#A93226';
        ctx.lineWidth = 2;
        ctx.strokeRect(-this.width / 2, -this.height / 2, this.width, this.height);
        
        // 炮管
        ctx.fillStyle = this.isPlayer ? '#2E5C8A' : '#A93226';
        ctx.fillRect(-3, -this.height / 2 - 10, 6, 15);
        
        // 坦克中心标记
        ctx.fillStyle = this.isPlayer ? '#ECF0F1' : '#F8B500';
        ctx.beginPath();
        ctx.arc(0, 0, 6, 0, Math.PI * 2);
        ctx.fill();
        
        ctx.restore();
    }

    move(dx, dy) {
        this.x += dx;
        this.y += dy;
    }

    canFire() {
        return Date.now() - this.lastFired > GAME_CONFIG.FIRE_COOLDOWN;
    }

    fire() {
        if (!this.canFire() || !this.alive) return null;
        
        this.lastFired = Date.now();
        
        let bulletX = this.x + this.width / 2;
        let bulletY = this.y + this.height / 2;
        
        // 根据方向调整子弹起始位置
        switch (this.direction) {
            case DIRECTION.UP:
                bulletY = this.y - GAME_CONFIG.BULLET_SIZE;
                break;
            case DIRECTION.DOWN:
                bulletY = this.y + this.height + GAME_CONFIG.BULLET_SIZE;
                break;
            case DIRECTION.LEFT:
                bulletX = this.x - GAME_CONFIG.BULLET_SIZE;
                break;
            case DIRECTION.RIGHT:
                bulletX = this.x + this.width + GAME_CONFIG.BULLET_SIZE;
                break;
        }
        
        return new Bullet(bulletX, bulletY, this.direction, this.isPlayer);
    }

    // AI行为
    updateAI(player, walls, otherEnemies) {
        if (!this.alive) return;

        const now = Date.now();
        
        // 定期改变方向
        if (now - this.lastDirectionChange > this.directionChangeCooldown) {
            this.direction = Math.floor(Math.random() * 4);
            this.lastDirectionChange = now;
            this.directionChangeCooldown = 1000 + Math.random() * 2000;
        }

        // 尝试向玩家方向移动（30%概率）
        if (Math.random() < 0.3) {
            const dx = player.x - this.x;
            const dy = player.y - this.y;
            
            if (Math.abs(dx) > Math.abs(dy)) {
                this.direction = dx > 0 ? DIRECTION.RIGHT : DIRECTION.LEFT;
            } else {
                this.direction = dy > 0 ? DIRECTION.DOWN : DIRECTION.UP;
            }
        }

        // 移动
        let newX = this.x;
        let newY = this.y;

        switch (this.direction) {
            case DIRECTION.UP:
                newY -= this.speed;
                break;
            case DIRECTION.DOWN:
                newY += this.speed;
                break;
            case DIRECTION.LEFT:
                newX -= this.speed;
                break;
            case DIRECTION.RIGHT:
                newX += this.speed;
                break;
        }

        // 检查边界
        if (newX >= 0 && newX + this.width <= GAME_CONFIG.CANVAS_WIDTH &&
            newY >= 0 && newY + this.height <= GAME_CONFIG.CANVAS_HEIGHT) {
            
            // 检查墙体碰撞
            const tempTank = { x: newX, y: newY, width: this.width, height: this.height };
            let collision = false;
            
            for (let wall of walls) {
                if (wall.alive && this.checkCollision(tempTank, wall)) {
                    collision = true;
                    break;
                }
            }

            if (!collision) {
                this.x = newX;
                this.y = newY;
            } else {
                // 碰到墙就改变方向
                this.direction = Math.floor(Math.random() * 4);
            }
        } else {
            // 碰到边界就改变方向
            this.direction = Math.floor(Math.random() * 4);
        }

        // 定期射击
        if (now - this.lastFiredTime > GAME_CONFIG.ENEMY_FIRE_INTERVAL) {
            this.lastFiredTime = now;
            return this.fire();
        }

        return null;
    }

    checkCollision(obj1, obj2) {
        return obj1.x < obj2.x + obj2.width &&
               obj1.x + obj1.width > obj2.x &&
               obj1.y < obj2.y + obj2.height &&
               obj1.y + obj1.height > obj2.y;
    }
}

// ==================== 子弹类 Bullet Class ====================
class Bullet {
    constructor(x, y, direction, isPlayerBullet) {
        this.x = x;
        this.y = y;
        this.width = GAME_CONFIG.BULLET_SIZE;
        this.height = GAME_CONFIG.BULLET_SIZE;
        this.direction = direction;
        this.speed = GAME_CONFIG.BULLET_SPEED;
        this.isPlayerBullet = isPlayerBullet;
        this.alive = true;
    }

    update() {
        if (!this.alive) return;

        switch (this.direction) {
            case DIRECTION.UP:
                this.y -= this.speed;
                break;
            case DIRECTION.DOWN:
                this.y += this.speed;
                break;
            case DIRECTION.LEFT:
                this.x -= this.speed;
                break;
            case DIRECTION.RIGHT:
                this.x += this.speed;
                break;
        }

        // 检查是否出界
        if (this.x < 0 || this.x > GAME_CONFIG.CANVAS_WIDTH ||
            this.y < 0 || this.y > GAME_CONFIG.CANVAS_HEIGHT) {
            this.alive = false;
        }
    }

    draw(ctx) {
        if (!this.alive) return;

        ctx.fillStyle = '#F8B500';
        ctx.shadowColor = '#F8B500';
        ctx.shadowBlur = 10;
        ctx.beginPath();
        ctx.arc(this.x, this.y, this.width, 0, Math.PI * 2);
        ctx.fill();
        ctx.shadowBlur = 0;
    }

    checkCollision(obj) {
        return this.alive &&
               this.x < obj.x + obj.width &&
               this.x + this.width > obj.x &&
               this.y < obj.y + obj.height &&
               this.y + this.height > obj.y;
    }
}

// ==================== 墙体类 Wall Class ====================
class Wall {
    constructor(x, y, destructible = true) {
        this.x = x;
        this.y = y;
        this.width = GAME_CONFIG.WALL_SIZE;
        this.height = GAME_CONFIG.WALL_SIZE;
        this.destructible = destructible;
        this.alive = true;
    }

    draw(ctx) {
        if (!this.alive) return;

        if (this.destructible) {
            // 砖墙
            ctx.fillStyle = '#8B4513';
            ctx.fillRect(this.x, this.y, this.width, this.height);
            ctx.strokeStyle = '#654321';
            ctx.lineWidth = 1;
            ctx.strokeRect(this.x, this.y, this.width, this.height);
            
            // 砖块纹理
            ctx.strokeStyle = '#654321';
            ctx.beginPath();
            ctx.moveTo(this.x, this.y + this.height / 2);
            ctx.lineTo(this.x + this.width, this.y + this.height / 2);
            ctx.stroke();
        } else {
            // 钢墙
            ctx.fillStyle = '#7F8C8D';
            ctx.fillRect(this.x, this.y, this.width, this.height);
            ctx.strokeStyle = '#5D6D7E';
            ctx.lineWidth = 2;
            ctx.strokeRect(this.x, this.y, this.width, this.height);
            
            // 金属质感
            ctx.fillStyle = '#95A5A6';
            ctx.fillRect(this.x + 2, this.y + 2, this.width / 3, this.height / 3);
        }
    }

    hit() {
        if (this.destructible) {
            this.alive = false;
            return true;
        }
        return false;
    }
}

// ==================== 基地类 Base Class ====================
class Base {
    constructor(x, y) {
        this.x = x;
        this.y = y;
        this.width = GAME_CONFIG.BASE_SIZE;
        this.height = GAME_CONFIG.BASE_SIZE;
        this.alive = true;
    }

    draw(ctx) {
        if (!this.alive) {
            // 被摧毁的基地
            ctx.fillStyle = '#C0392B';
            ctx.fillRect(this.x, this.y, this.width, this.height);
            
            // 爆炸效果
            ctx.strokeStyle = '#E74C3C';
            ctx.lineWidth = 3;
            ctx.beginPath();
            ctx.moveTo(this.x, this.y);
            ctx.lineTo(this.x + this.width, this.y + this.height);
            ctx.moveTo(this.x + this.width, this.y);
            ctx.lineTo(this.x, this.y + this.height);
            ctx.stroke();
        } else {
            // 正常基地
            ctx.fillStyle = '#27AE60';
            ctx.fillRect(this.x, this.y, this.width, this.height);
            ctx.strokeStyle = '#1E8449';
            ctx.lineWidth = 3;
            ctx.strokeRect(this.x, this.y, this.width, this.height);
            
            // 基地图案
            ctx.fillStyle = '#1E8449';
            ctx.beginPath();
            ctx.moveTo(this.x + this.width / 2, this.y + 5);
            ctx.lineTo(this.x + this.width - 5, this.y + this.height - 5);
            ctx.lineTo(this.x + 5, this.y + this.height - 5);
            ctx.closePath();
            ctx.fill();
        }
    }

    hit() {
        this.alive = false;
    }
}

// ==================== 游戏主类 Game Class ====================
class Game {
    constructor() {
        this.canvas = document.getElementById('gameCanvas');
        this.ctx = this.canvas.getContext('2d');
        this.canvas.width = GAME_CONFIG.CANVAS_WIDTH;
        this.canvas.height = GAME_CONFIG.CANVAS_HEIGHT;

        // 游戏状态
        this.player = null;
        this.enemies = [];
        this.bullets = [];
        this.walls = [];
        this.base = null;
        this.lives = GAME_CONFIG.INITIAL_LIVES;
        this.score = 0;
        this.level = 1;
        this.enemiesRemaining = 0;
        this.gameRunning = false;
        this.paused = false;

        // 输入处理
        this.keys = {};
        this.setupEventListeners();

        // 初始化游戏
        this.init();
    }

    setupEventListeners() {
        // 键盘事件
        window.addEventListener('keydown', (e) => {
            this.keys[e.key] = true;
            
            // 射击
            if (e.key === ' ' && this.gameRunning && !this.paused) {
                e.preventDefault();
                const bullet = this.player.fire();
                if (bullet) {
                    this.bullets.push(bullet);
                }
            }
            
            // 暂停
            if (e.key === 'p' || e.key === 'P') {
                this.togglePause();
            }
        });

        window.addEventListener('keyup', (e) => {
            this.keys[e.key] = false;
        });

        // 重新开始按钮
        document.getElementById('restartButton').addEventListener('click', () => {
            this.restart();
        });
    }

    init() {
        // 创建玩家
        this.player = new Tank(
            GAME_CONFIG.CANVAS_WIDTH / 2 - GAME_CONFIG.TANK_SIZE / 2,
            GAME_CONFIG.CANVAS_HEIGHT - GAME_CONFIG.TANK_SIZE - 10,
            true
        );

        // 创建基地
        this.base = new Base(
            GAME_CONFIG.CANVAS_WIDTH / 2 - GAME_CONFIG.BASE_SIZE / 2,
            GAME_CONFIG.CANVAS_HEIGHT - GAME_CONFIG.BASE_SIZE - 60
        );

        // 生成关卡
        this.generateLevel();

        // 更新UI
        this.updateUI();

        // 开始游戏循环
        this.gameRunning = true;
        this.gameLoop();
    }

    generateLevel() {
        // 清空现有元素
        this.enemies = [];
        this.bullets = [];
        this.walls = [];
        
        // 设置当前关卡的敌人总数
        this.enemiesRemaining = GAME_CONFIG.ENEMIES_PER_LEVEL + (this.level - 1) * 2;

        // 生成初始敌人（不超过最大同屏数量）
        const initialEnemies = Math.min(GAME_CONFIG.MAX_ENEMIES_ON_SCREEN, this.enemiesRemaining);
        for (let i = 0; i < initialEnemies; i++) {
            this.spawnEnemy();
        }

        // 生成墙体布局
        this.generateWalls();
    }

    generateWalls() {
        const wallPatterns = [
            // 基地周围的保护墙
            { x: this.base.x - GAME_CONFIG.WALL_SIZE, y: this.base.y, destructible: false },
            { x: this.base.x + this.base.width, y: this.base.y, destructible: false },
            { x: this.base.x - GAME_CONFIG.WALL_SIZE, y: this.base.y + GAME_CONFIG.WALL_SIZE, destructible: true },
            { x: this.base.x + this.base.width, y: this.base.y + GAME_CONFIG.WALL_SIZE, destructible: true },
        ];

        // 随机生成障碍物
        const numObstacles = 15 + this.level * 3;
        for (let i = 0; i < numObstacles; i++) {
            const x = Math.floor(Math.random() * (GAME_CONFIG.CANVAS_WIDTH / GAME_CONFIG.WALL_SIZE)) * GAME_CONFIG.WALL_SIZE;
            const y = Math.floor(Math.random() * (GAME_CONFIG.CANVAS_HEIGHT * 0.6 / GAME_CONFIG.WALL_SIZE)) * GAME_CONFIG.WALL_SIZE;
            
            // 避免在玩家和基地附近生成
            if (y < GAME_CONFIG.CANVAS_HEIGHT - 150) {
                const destructible = Math.random() > 0.3;
                wallPatterns.push({ x, y, destructible });
            }
        }

        // 创建墙体
        wallPatterns.forEach(pattern => {
            this.walls.push(new Wall(pattern.x, pattern.y, pattern.destructible));
        });
    }

    spawnEnemy() {
        if (this.enemiesRemaining <= 0) return;

        const spawnPositions = [
            { x: 50, y: 50 },
            { x: GAME_CONFIG.CANVAS_WIDTH / 2 - GAME_CONFIG.TANK_SIZE / 2, y: 50 },
            { x: GAME_CONFIG.CANVAS_WIDTH - 50 - GAME_CONFIG.TANK_SIZE, y: 50 }
        ];

        const pos = spawnPositions[Math.floor(Math.random() * spawnPositions.length)];
        const enemy = new Tank(pos.x, pos.y, false);
        this.enemies.push(enemy);
        this.enemiesRemaining--;
    }

    handlePlayerMovement() {
        if (!this.player.alive || this.paused) return;

        let newX = this.player.x;
        let newY = this.player.y;
        let moved = false;

        if (this.keys['ArrowUp']) {
            this.player.direction = DIRECTION.UP;
            newY -= this.player.speed;
            moved = true;
        } else if (this.keys['ArrowDown']) {
            this.player.direction = DIRECTION.DOWN;
            newY += this.player.speed;
            moved = true;
        } else if (this.keys['ArrowLeft']) {
            this.player.direction = DIRECTION.LEFT;
            newX -= this.player.speed;
            moved = true;
        } else if (this.keys['ArrowRight']) {
            this.player.direction = DIRECTION.RIGHT;
            newX += this.player.speed;
            moved = true;
        }

        if (moved) {
            // 边界检测
            if (newX >= 0 && newX + this.player.width <= GAME_CONFIG.CANVAS_WIDTH &&
                newY >= 0 && newY + this.player.height <= GAME_CONFIG.CANVAS_HEIGHT) {
                
                // 墙体碰撞检测
                const tempPlayer = { 
                    x: newX, 
                    y: newY, 
                    width: this.player.width, 
                    height: this.player.height 
                };
                
                let collision = false;
                for (let wall of this.walls) {
                    if (wall.alive && this.checkCollision(tempPlayer, wall)) {
                        collision = true;
                        break;
                    }
                }

                // 基地碰撞检测
                if (this.base.alive && this.checkCollision(tempPlayer, this.base)) {
                    collision = true;
                }

                if (!collision) {
                    this.player.x = newX;
                    this.player.y = newY;
                }
            }
        }
    }

    updateEnemies() {
        if (this.paused) return;

        // 更新现有敌人
        for (let enemy of this.enemies) {
            if (enemy.alive) {
                const bullet = enemy.updateAI(this.player, this.walls, this.enemies);
                if (bullet) {
                    this.bullets.push(bullet);
                }
            }
        }

        // 如果场上敌人少于最大数量，且还有剩余敌人，则生成新敌人
        const aliveEnemies = this.enemies.filter(e => e.alive).length;
        if (aliveEnemies < GAME_CONFIG.MAX_ENEMIES_ON_SCREEN && this.enemiesRemaining > 0) {
            this.spawnEnemy();
        }

        // 移除死亡的敌人
        this.enemies = this.enemies.filter(e => e.alive);
    }

    updateBullets() {
        if (this.paused) return;

        for (let bullet of this.bullets) {
            if (!bullet.alive) continue;

            bullet.update();

            // 检查子弹与墙的碰撞
            for (let wall of this.walls) {
                if (wall.alive && bullet.checkCollision(wall)) {
                    wall.hit();
                    bullet.alive = false;
                    break;
                }
            }

            // 检查子弹与坦克的碰撞
            if (bullet.isPlayerBullet) {
                // 玩家子弹击中敌人
                for (let enemy of this.enemies) {
                    if (enemy.alive && bullet.checkCollision(enemy)) {
                        enemy.alive = false;
                        bullet.alive = false;
                        this.score += 100;
                        this.updateUI();
                        break;
                    }
                }
            } else {
                // 敌人子弹击中玩家
                if (this.player.alive && bullet.checkCollision(this.player)) {
                    this.playerHit();
                    bullet.alive = false;
                }

                // 敌人子弹击中基地
                if (this.base.alive && bullet.checkCollision(this.base)) {
                    this.base.hit();
                    bullet.alive = false;
                    this.gameOver(false);
                }
            }
        }

        // 移除死亡的子弹
        this.bullets = this.bullets.filter(b => b.alive);
    }

    playerHit() {
        this.lives--;
        this.updateUI();

        if (this.lives <= 0) {
            this.player.alive = false;
            this.gameOver(false);
        } else {
            // 重置玩家位置
            this.player.x = GAME_CONFIG.CANVAS_WIDTH / 2 - GAME_CONFIG.TANK_SIZE / 2;
            this.player.y = GAME_CONFIG.CANVAS_HEIGHT - GAME_CONFIG.TANK_SIZE - 10;
            this.player.direction = DIRECTION.UP;
        }
    }

    checkLevelComplete() {
        if (this.enemies.length === 0 && this.enemiesRemaining === 0) {
            this.level++;
            this.score += 500;
            this.updateUI();
            
            // 短暂延迟后开始新关卡
            setTimeout(() => {
                if (this.gameRunning) {
                    this.generateLevel();
                }
            }, 1000);
        }
    }

    checkCollision(obj1, obj2) {
        return obj1.x < obj2.x + obj2.width &&
               obj1.x + obj1.width > obj2.x &&
               obj1.y < obj2.y + obj2.height &&
               obj1.y + obj1.height > obj2.y;
    }

    updateUI() {
        document.getElementById('lives').textContent = this.lives;
        document.getElementById('score').textContent = this.score;
        document.getElementById('enemies').textContent = this.enemies.length + this.enemiesRemaining;
        document.getElementById('level').textContent = this.level;
    }

    draw() {
        // 清空画布
        this.ctx.fillStyle = '#1a1a1a';
        this.ctx.fillRect(0, 0, GAME_CONFIG.CANVAS_WIDTH, GAME_CONFIG.CANVAS_HEIGHT);

        // 绘制网格背景
        this.ctx.strokeStyle = '#2a2a2a';
        this.ctx.lineWidth = 1;
        for (let i = 0; i < GAME_CONFIG.CANVAS_WIDTH; i += 40) {
            this.ctx.beginPath();
            this.ctx.moveTo(i, 0);
            this.ctx.lineTo(i, GAME_CONFIG.CANVAS_HEIGHT);
            this.ctx.stroke();
        }
        for (let i = 0; i < GAME_CONFIG.CANVAS_HEIGHT; i += 40) {
            this.ctx.beginPath();
            this.ctx.moveTo(0, i);
            this.ctx.lineTo(GAME_CONFIG.CANVAS_WIDTH, i);
            this.ctx.stroke();
        }

        // 绘制游戏元素
        this.base.draw(this.ctx);
        this.walls.forEach(wall => wall.draw(this.ctx));
        this.player.draw(this.ctx);
        this.enemies.forEach(enemy => enemy.draw(this.ctx));
        this.bullets.forEach(bullet => bullet.draw(this.ctx));

        // 暂停提示
        if (this.paused) {
            this.ctx.fillStyle = 'rgba(0, 0, 0, 0.5)';
            this.ctx.fillRect(0, 0, GAME_CONFIG.CANVAS_WIDTH, GAME_CONFIG.CANVAS_HEIGHT);
            
            this.ctx.fillStyle = 'white';
            this.ctx.font = 'bold 48px Arial';
            this.ctx.textAlign = 'center';
            this.ctx.fillText('暂停 PAUSED', GAME_CONFIG.CANVAS_WIDTH / 2, GAME_CONFIG.CANVAS_HEIGHT / 2);
            this.ctx.font = '24px Arial';
            this.ctx.fillText('按 P 继续游戏', GAME_CONFIG.CANVAS_WIDTH / 2, GAME_CONFIG.CANVAS_HEIGHT / 2 + 50);
        }
    }

    gameLoop() {
        if (!this.gameRunning) return;

        if (!this.paused) {
            this.handlePlayerMovement();
            this.updateEnemies();
            this.updateBullets();
            this.checkLevelComplete();
        }

        this.draw();
        requestAnimationFrame(() => this.gameLoop());
    }

    togglePause() {
        if (!this.gameRunning) return;
        this.paused = !this.paused;
    }

    gameOver(victory) {
        this.gameRunning = false;
        
        const overlay = document.getElementById('gameOverlay');
        const title = document.getElementById('overlayTitle');
        const message = document.getElementById('overlayMessage');

        if (victory) {
            title.textContent = '🎉 胜利！Victory!';
            message.textContent = `恭喜通关！最终得分：${this.score}`;
        } else {
            title.textContent = '💥 游戏结束 Game Over';
            message.textContent = `最终得分：${this.score} | 到达关卡：${this.level}`;
        }

        overlay.classList.remove('hidden');
    }

    restart() {
        // 重置游戏状态
        this.lives = GAME_CONFIG.INITIAL_LIVES;
        this.score = 0;
        this.level = 1;
        this.enemiesRemaining = 0;
        this.paused = false;

        // 隐藏overlay
        document.getElementById('gameOverlay').classList.add('hidden');

        // 重新初始化
        this.init();
    }
}

// ==================== 游戏启动 Game Start ====================
window.addEventListener('load', () => {
    new Game();
});
