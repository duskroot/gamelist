type SfxName = "deal" | "slide" | "dice" | "chip" | "click" | "win";

const FILES: Record<SfxName, string[]> = {
  deal: ["/game/sfx/cardPlace1.ogg", "/game/sfx/cardPlace2.ogg", "/game/sfx/cardPlace3.ogg"],
  slide: ["/game/sfx/cardSlide1.ogg", "/game/sfx/cardSlide2.ogg"],
  dice: ["/game/sfx/dieThrow1.ogg", "/game/sfx/dieThrow2.ogg", "/game/sfx/dieShuffle1.ogg"],
  chip: ["/game/sfx/chipsCollide1.ogg", "/game/sfx/chipsCollide2.ogg"],
  click: ["/game/sfx/cardPlace1.ogg"],
  win: ["/game/sfx/chipsCollide3.ogg"],
};

let muted = false;
let unlocked = false;

export function setMuted(next: boolean): void {
  muted = next;
  localStorage.setItem("kaiju.muted", muted ? "1" : "0");
}

export function isMuted(): boolean {
  return muted;
}

export function loadMutePreference(): void {
  muted = localStorage.getItem("kaiju.muted") === "1";
}

export function unlockAudio(): void {
  if (unlocked) return;
  unlocked = true;
  // play near-silent buffer to unlock mobile audio
  play("click", 0.01);
}

export function play(name: SfxName, volume = 0.45): void {
  if (muted || typeof Audio === "undefined") return;
  const list = FILES[name];
  if (!list?.length) return;
  const src = list[Math.floor(Math.random() * list.length)];
  try {
    const audio = new Audio(src);
    audio.volume = volume;
    void audio.play().catch(() => {
      /* autoplay restrictions */
    });
  } catch {
    /* ignore */
  }
}
