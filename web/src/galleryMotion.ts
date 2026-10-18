import { useLayoutEffect, useRef, useState } from "react";

const duration = 240;
const reduced = () =>
  window.matchMedia("(prefers-reduced-motion: reduce)").matches;
function thumbnail(id: string) {
  const card = document.querySelector<HTMLElement>(
    `.file-card[data-item-id="${CSS.escape(id)}"]`,
  );
  const image = card?.querySelector("img");
  const rect = (image || card)?.getBoundingClientRect();
  if (!rect || rect.bottom <= 0 || rect.top >= innerHeight || !rect.width)
    return;
  return {
    rect,
    src: image?.currentSrc,
    width: image?.naturalWidth || rect.width,
    height: image?.naturalHeight || rect.height,
  };
}

// Capture the thumbnail geometry before the gallery takes focus.
export function useGalleryMotion(
  id: string,
  close: () => void,
  flyThumbnail: boolean,
) {
  const [source] = useState(() => thumbnail(id));
  const root = useRef<HTMLElement>(null);
  const media = useRef<HTMLDivElement>(null);
  const animations = useRef<Animation[]>([]);
  const ghost = useRef<HTMLImageElement | null>(null);
  const closing = useRef(false);
  const generation = useRef(0);
  const mounted = useRef(true);
  function clear() {
    generation.current++;
    animations.current.forEach((animation) => animation.cancel());
    animations.current = [];
    ghost.current?.remove();
    ghost.current = null;
  }
  function animate(exiting: boolean) {
    clear();
    if (reduced() || !root.current?.animate || !media.current)
      return Promise.resolve();
    const options = {
      duration,
      easing: "cubic-bezier(.2,.7,.2,1)",
      fill: "both" as const,
    };
    animations.current.push(
      root.current.animate(
        [{ opacity: exiting ? 1 : 0 }, { opacity: exiting ? 0 : 1 }],
        options,
      ),
    );
    const target = exiting ? thumbnail(id) : source;
    const image =
      media.current.querySelector<HTMLImageElement>(".detail-image");
    const src = exiting ? image?.currentSrc : target?.src;
    // Readers have their own page geometry and asynchronous paint lifecycle.
    if (flyThumbnail && target && src) {
      const bounds = media.current.getBoundingClientRect();
      const styles = getComputedStyle(media.current);
      const availableWidth =
        bounds.width -
        parseFloat(styles.paddingLeft) -
        parseFloat(styles.paddingRight);
      const availableHeight =
        bounds.height -
        parseFloat(styles.paddingTop) -
        parseFloat(styles.paddingBottom);
      const width = image?.naturalWidth || target.width;
      const height = image?.naturalHeight || target.height;
      const scale = Math.min(availableWidth / width, availableHeight / height);
      const expanded = {
        left: bounds.left + (bounds.width - width * scale) / 2,
        top: bounds.top + (bounds.height - height * scale) / 2,
        width: width * scale,
        height: height * scale,
      };
      const frame = (rect: typeof expanded) => ({
        left: `${rect.left}px`,
        top: `${rect.top}px`,
        width: `${rect.width}px`,
        height: `${rect.height}px`,
      });
      const copy = document.createElement("img");
      copy.src = src;
      copy.alt = "";
      copy.className = "gallery-flight";
      copy.setAttribute("aria-hidden", "true");
      document.body.append(copy);
      ghost.current = copy;
      const frames = [frame(target.rect), frame(expanded)];
      animations.current.push(
        copy.animate(exiting ? frames.reverse() : frames, options),
      );
      animations.current.push(
        media.current.animate([{ opacity: 0 }, { opacity: 0 }], options),
      );
    }
    return Promise.all(
      animations.current.map((animation) => animation.finished.catch(() => {})),
    ).then(() => {});
  }
  useLayoutEffect(() => {
    mounted.current = true;
    const opening = animate(false);
    const token = generation.current;
    void opening.then(() => {
      if (!closing.current && token === generation.current) clear();
    });
    return () => {
      mounted.current = false;
      clear();
    };
  }, []);
  return {
    root,
    media,
    dismiss: async () => {
      if (closing.current) return;
      closing.current = true;
      await animate(true);
      if (mounted.current) close();
    },
  };
}
