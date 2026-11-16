import {
  _decorator, Component, Node, Label, Button, Color, UITransform, Widget,
  Sprite, SpriteFrame, resources, AudioSource, tween, Vec3,
  EditBox, view, ResolutionPolicy, Graphics,
} from 'cc';
import { API_BASE } from './Config';
import { Api } from './net/Api';
import { Http } from './net/Http';
import { ApiError, RoomView, Snapshot } from './net/Types';
import { clearToken, getLoginCode, loadNickname, loadToken, saveNickname, saveToken } from './platform/Auth';
import { PlayLoop } from './play/PlayLoop';
import { GAMES, gameName } from './games/Meta';
import { cardBackPath, cardFacePath, dicePath, feltPath } from './games/AssetMap';
import type { Card } from './net/Types';

const { ccclass, property } = _decorator;

type Screen = 'login' | 'lobby' | 'room' | 'play';

@ccclass('GameApp')
export class GameApp extends Component {
  @property
  apiBase = API_BASE;

  private uiRoot: Node | null = null;
  private toastLabel: Label | null = null;
  private api!: Api;
  private loop!: PlayLoop;
  private token = '';
  private screen: Screen = 'login';
  private room: RoomView | null = null;
  private snap: Snapshot | null = null;
  private selectedGame = 'dice';
  private nick = '';
  private joinCode = '';
  private frameCache = new Map<string, SpriteFrame>();
  private audio: AudioSource | null = null;

  onLoad() {
    view.setDesignResolutionSize(750, 1334, ResolutionPolicy.FIXED_WIDTH);
    this.token = loadToken();
    this.nick = loadNickname();
    const http = new Http({
      baseUrl: this.apiBase,
      getToken: () => this.token,
      onUnauthorized: () => {
        clearToken();
        this.token = '';
        this.goto('login');
        this.toast('登录失效', 'error');
      },
    });
    this.api = new Api(http);
    this.loop = new PlayLoop(this.api, {
      onSnapshot: (snap) => {
        this.snap = snap;
        if (this.screen === 'play') this.render();
      },
      onToast: (msg, tone) => this.toast(msg, tone),
    });
    this.buildChrome();
    void this.bootstrap();
  }

  onDestroy() {
    this.loop?.stop();
  }

  private buildChrome() {
    this.node.removeAllChildren();
    const canvas = this.node;
    const ut = canvas.getComponent(UITransform) || canvas.addComponent(UITransform);
    ut.setContentSize(750, 1334);

    this.uiRoot = new Node('UIRoot');
    this.uiRoot.addComponent(UITransform).setContentSize(750, 1334);
    const widget = this.uiRoot.addComponent(Widget);
    widget.isAlignTop = widget.isAlignBottom = widget.isAlignLeft = widget.isAlignRight = true;
    widget.top = widget.bottom = widget.left = widget.right = 0;
    canvas.addChild(this.uiRoot);

    const toastNode = new Node('Toast');
    toastNode.addComponent(UITransform).setContentSize(600, 48);
    toastNode.setPosition(0, -560, 0);
    this.toastLabel = toastNode.addComponent(Label);
    this.toastLabel.string = '';
    this.toastLabel.fontSize = 28;
    this.toastLabel.color = new Color(244, 239, 228);
    this.uiRoot.addChild(toastNode);

    this.audio = this.node.addComponent(AudioSource);
  }

  private async bootstrap() {
    try {
      await this.api.describeHealth();
    } catch {
      this.toast('后端未就绪，请检查 API_BASE', 'error');
    }
    if (this.token) {
      try {
        await this.api.describeSession();
        this.goto('lobby');
        return;
      } catch {
        clearToken();
        this.token = '';
      }
    }
    this.goto('login');
  }

  private goto(screen: Screen) {
    this.screen = screen;
    this.render();
  }

  private clearUI() {
    if (!this.uiRoot) return;
    const keep = new Set(['Toast']);
    [...this.uiRoot.children].forEach((c) => {
      if (!keep.has(c.name)) c.destroy();
    });
  }

  private render() {
    this.clearUI();
    if (this.screen === 'login') this.renderLogin();
    else if (this.screen === 'lobby') this.renderLobby();
    else if (this.screen === 'room') this.renderRoom();
    else if (this.screen === 'play') this.renderPlay();
  }

  private panel(y: number, h: number, title?: string) {
    const n = new Node(title || 'Panel');
    n.addComponent(UITransform).setContentSize(690, h);
    n.setPosition(0, y, 0);
    const g = n.addComponent(Graphics);
    g.fillColor = new Color(10, 50, 38, 210);
    g.roundRect(-345, -h / 2, 690, h, 24);
    g.fill();
    if (title) {
      const t = new Node('Title');
      t.setPosition(-300, h / 2 - 36, 0);
      const lab = t.addComponent(Label);
      lab.string = title;
      lab.fontSize = 34;
      lab.color = new Color(232, 180, 90);
      lab.horizontalAlign = Label.HorizontalAlign.LEFT;
      t.addComponent(UITransform).setContentSize(400, 40);
      n.addChild(t);
    }
    this.uiRoot!.addChild(n);
    return n;
  }

  private addLabel(parent: Node, text: string, x: number, y: number, size = 26, color = new Color(244, 239, 228)) {
    const n = new Node('Label');
    n.setPosition(x, y, 0);
    const lab = n.addComponent(Label);
    lab.string = text;
    lab.fontSize = size;
    lab.color = color;
    lab.overflow = Label.Overflow.RESIZE_HEIGHT;
    n.addComponent(UITransform).setContentSize(620, size + 8);
    parent.addChild(n);
    return lab;
  }

  private addButton(parent: Node, text: string, x: number, y: number, w: number, onClick: () => void, tone: 'gold' | 'mint' | 'ghost' = 'gold') {
    const n = new Node(`Btn_${text}`);
    n.setPosition(x, y, 0);
    n.addComponent(UITransform).setContentSize(w, 64);
    const g = n.addComponent(Graphics);
    if (tone === 'gold') g.fillColor = new Color(232, 180, 90);
    else if (tone === 'mint') g.fillColor = new Color(125, 206, 176);
    else g.fillColor = new Color(255, 255, 255, 28);
    g.roundRect(-w / 2, -32, w, 64, 32);
    g.fill();
    const labN = new Node('t');
    const lab = labN.addComponent(Label);
    lab.string = text;
    lab.fontSize = 28;
    lab.color = tone === 'ghost' ? new Color(244, 239, 228) : new Color(28, 20, 8);
    labN.addComponent(UITransform).setContentSize(w, 64);
    n.addChild(labN);
    n.addComponent(Button);
    n.on(Button.EventType.CLICK, onClick, this);
    parent.addChild(n);
  }

  private renderLogin() {
    const brand = new Node('Brand');
    brand.setPosition(0, 420, 0);
    const lab = brand.addComponent(Label);
    lab.string = '开局';
    lab.fontSize = 96;
    lab.color = new Color(232, 180, 90);
    brand.addComponent(UITransform).setContentSize(400, 110);
    this.uiRoot!.addChild(brand);

    void this.attachSprite(this.uiRoot!, feltPath(), 0, 180, 520, 280);

    const panel = this.panel(-180, 420, '进入牌桌');
    this.addLabel(panel, 'Cocos 成品客户端 · Kenney CC0 贴图', 0, 120, 22, new Color(201, 192, 174));

    const box = new Node('NickBox');
    box.setPosition(0, 30, 0);
    box.addComponent(UITransform).setContentSize(560, 70);
    const edit = box.addComponent(EditBox);
    edit.placeholder = '昵称';
    edit.string = this.nick;
    edit.maxLength = 32;
    edit.textLabel && (edit.textLabel.fontSize = 28);
    panel.addChild(box);

    this.addButton(panel, '开始玩', 0, -90, 280, () => {
      this.nick = edit.string?.trim() || this.nick;
      void this.doLogin();
    });
  }

  private async doLogin() {
    try {
      saveNickname(this.nick);
      const code = await getLoginCode();
      const res = await this.api.loginByWechat({ Code: code, Nickname: this.nick || undefined });
      this.token = res.Token;
      saveToken(res.Token);
      this.toast(`欢迎，${res.Nickname || '玩家'}`, 'ok');
      this.goto('lobby');
    } catch (err) {
      this.toast(err instanceof ApiError ? err.message : '登录失败', 'error');
    }
  }

  private renderLobby() {
    this.addLabel(this.uiRoot!, '大厅', 0, 580, 40, new Color(232, 180, 90));
    this.addButton(this.uiRoot!, '退出', 280, 580, 140, () => {
      clearToken(); this.token = ''; this.loop.stop(); this.goto('login');
    }, 'ghost');

    const panel = this.panel(40, 980, '创建房间');
    let y = 360;
    for (const g of GAMES) {
      const selected = g.code === this.selectedGame;
      const row = new Node(g.code);
      row.setPosition(0, y, 0);
      row.addComponent(UITransform).setContentSize(620, 78);
      const gg = row.addComponent(Graphics);
      gg.fillColor = selected ? new Color(232, 180, 90, 50) : new Color(255, 255, 255, 16);
      gg.roundRect(-310, -36, 620, 72, 16);
      gg.fill();
      if (selected) {
        gg.strokeColor = new Color(232, 180, 90);
        gg.lineWidth = 3;
        gg.roundRect(-310, -36, 620, 72, 16);
        gg.stroke();
      }
      this.addLabel(row, `${g.name}  ·  ${g.min}-${g.max}人`, -280, 0, 28);
      this.addLabel(row, g.blurb, 40, 0, 22, new Color(201, 192, 174));
      row.addComponent(Button);
      row.on(Button.EventType.CLICK, () => {
        this.selectedGame = g.code;
        this.render();
      }, this);
      panel.addChild(row);
      y -= 86;
    }

    const joinBox = new Node('JoinCode');
    joinBox.setPosition(0, -360, 0);
    joinBox.addComponent(UITransform).setContentSize(560, 64);
    const joinEdit = joinBox.addComponent(EditBox);
    joinEdit.placeholder = '6 位房间号';
    joinEdit.string = this.joinCode;
    joinEdit.maxLength = 6;
    joinEdit.inputMode = EditBox.InputMode.NUMERIC;
    panel.addChild(joinBox);

    this.addButton(panel, '创建并进入', -150, -450, 260, () => void this.createRoom(), 'gold');
    this.addButton(panel, '加入', 150, -450, 200, () => {
      this.joinCode = (joinEdit.string || '').trim();
      void this.promptJoin();
    }, 'mint');
  }

  private async createRoom() {
    const meta = GAMES.find((g) => g.code === this.selectedGame)!;
    try {
      this.room = await this.api.createRoom({
        GameCode: meta.code,
        Variant: meta.variants[0],
        MaxSeats: Math.min(6, meta.max),
        Config: meta.defaultConfig,
      });
      this.toast(`房间 ${this.room.Code}`, 'ok');
      this.goto('room');
    } catch (err) {
      this.toast(err instanceof ApiError ? err.message : '创建失败', 'error');
    }
  }

  private async promptJoin() {
    const code = this.joinCode || '';
    if (!/^\d{6}$/.test(code)) {
      this.toast('请输入 6 位房间号', 'error');
      return;
    }
    try {
      this.room = await this.api.joinRoom({ Code: code });
      this.goto('room');
    } catch (err) {
      this.toast(err instanceof ApiError ? err.message : '加入失败', 'error');
    }
  }

  private renderRoom() {
    const room = this.room;
    if (!room) { this.goto('lobby'); return; }
    const panel = this.panel(40, 900, gameName(room.GameCode));
    this.addLabel(panel, `房间号 ${room.Code} · ${room.Status}`, 0, 340, 28, new Color(201, 192, 174));
    let y = 250;
    for (const m of room.Members) {
      this.addLabel(
        panel,
        `${m.Seat}. ${m.Nickname || m.UserId}  ${m.Ready ? '已准备' : '未准备'}${m.UserId === room.HostUserId ? ' · 房主' : ''}`,
        0, y, 28,
      );
      y -= 56;
    }
    this.addButton(panel, '准备/取消', -180, -280, 220, () => void this.toggleReady(), 'mint');
    this.addButton(panel, '开始游戏', 140, -280, 220, () => void this.startPlay());
    this.addButton(panel, '返回大厅', 0, -370, 220, () => { this.loop.stop(); this.room = null; this.goto('lobby'); }, 'ghost');
    this.addLabel(panel, `分享房间号 ${room.Code}`, 0, -450, 26, new Color(232, 180, 90));
  }

  private async toggleReady() {
    if (!this.room) return;
    try {
      const session = await this.api.describeSession();
      const me = this.room.Members.find((m) => m.UserId === session.UserId);
      this.room = await this.api.setReady({ RoomId: this.room.RoomId, Ready: !me?.Ready });
      this.render();
    } catch (err) {
      this.toast(err instanceof ApiError ? err.message : '准备失败', 'error');
    }
  }

  private async startPlay() {
    if (!this.room) return;
    try {
      this.snap = await this.api.startPlay({ RoomId: this.room.RoomId });
      this.room = await this.api.describeRoom({ RoomId: this.room.RoomId });
      this.loop.start(this.snap.PlayId, this.room.RoomId, this.snap.Cursor);
      this.goto('play');
    } catch (err) {
      this.toast(err instanceof ApiError ? err.message : '开局失败', 'error');
    }
  }

  private renderPlay() {
    const snap = this.snap;
    if (!snap) { this.goto('room'); return; }
    void this.attachSprite(this.uiRoot!, feltPath(), 0, 80, 700, 900);

    const panel = this.panel(40, 1100, `${gameName(snap.GameCode)} · ${snap.Phase}`);
    this.addLabel(panel, `座位 ${snap.Seat}${snap.Settled ? ' · 已结算' : ''}`, 0, 460, 26, new Color(201, 192, 174));

    // public summary
    const summary = JSON.stringify(snap.Public || {}).slice(0, 120);
    this.addLabel(panel, summary, 0, 400, 20, new Color(201, 192, 174));

    // private cards
    const cards = (snap.Private?.Cards as Card[] | undefined) || [];
    let x = -((cards.length - 1) * 50);
    cards.forEach((c, i) => {
      void this.attachSprite(panel, cardFacePath(c), x + i * 100, 180, 110, 154).then((node) => {
        if (node) tween(node).set({ scale: new Vec3(0.6, 0.6, 1) }).to(0.25, { scale: new Vec3(1, 1, 1) }).start();
      });
    });
    if (!cards.length && snap.GameCode === 'zhajinhua') {
      void this.attachSprite(panel, cardBackPath(), 0, 180, 110, 154);
    }

    // dice preview from public
    const rolls = (snap.Public?.Rolls as { Seat: number; Dice: number[]; Sum: number }[] | undefined) || [];
    let dy = 40;
    for (const r of rolls.slice(-3)) {
      this.addLabel(panel, `${r.Seat}号 = ${r.Sum}`, -220, dy, 24);
      r.Dice.forEach((d, i) => {
        void this.attachSprite(panel, dicePath(d, i % 2 ? 'red' : 'white'), -40 + i * 70, dy, 56, 56);
      });
      dy -= 70;
    }

    let bx = -220;
    const legal = snap.Legal || [];
    if (snap.Settled) {
      this.addButton(panel, '回到房间', 0, -460, 240, () => {
        this.loop.stop();
        this.snap = null;
        this.goto('room');
      }, 'mint');
    } else if (!legal.length) {
      this.addLabel(panel, '等待其他玩家…', 0, -460, 26, new Color(201, 192, 174));
    } else {
      for (const a of legal.slice(0, 4)) {
        this.addButton(panel, a.Hint || a.Type, bx, -460, 150, () => {
          void this.loop.submit(a.Type, undefined, snap.Seat);
        }, 'gold');
        bx += 160;
      }
    }
  }

  private async attachSprite(parent: Node, path: string, x: number, y: number, w: number, h: number) {
    const n = new Node(path);
    n.setPosition(x, y, 0);
    n.addComponent(UITransform).setContentSize(w, h);
    const sp = n.addComponent(Sprite);
    sp.sizeMode = Sprite.SizeMode.CUSTOM;
    parent.addChild(n);
    try {
      const frame = await this.loadFrame(path);
      if (frame && n.isValid) {
        sp.spriteFrame = frame;
      }
    } catch {
      // missing texture
    }
    return n;
  }

  private loadFrame(path: string): Promise<SpriteFrame | null> {
    if (this.frameCache.has(path)) return Promise.resolve(this.frameCache.get(path)!);
    return new Promise((resolve) => {
      resources.load(`${path}/spriteFrame`, SpriteFrame, (err, frame) => {
        if (err || !frame) {
          // fallback: load texture path without /spriteFrame for some importers
          resources.load(path, SpriteFrame, (err2, frame2) => {
            if (!err2 && frame2) {
              this.frameCache.set(path, frame2);
              resolve(frame2);
            } else resolve(null);
          });
          return;
        }
        this.frameCache.set(path, frame);
        resolve(frame);
      });
    });
  }

  private toast(msg: string, _tone: 'info' | 'error' | 'ok' = 'info') {
    if (!this.toastLabel) return;
    this.toastLabel.string = msg;
    this.scheduleOnce(() => {
      if (this.toastLabel && this.toastLabel.string === msg) this.toastLabel.string = '';
    }, 2.4);
  }
}
