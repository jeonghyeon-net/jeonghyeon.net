import React, { useCallback } from "react";

type ResizeHandleProps = {
  direction: "horizontal" | "vertical";
  value: number;
  min: number;
  max: number;
  invert?: boolean; // true = dragging down/right decreases value (for terminal height)
  onChange: (value: number) => void;
};

export default function ResizeHandle({ direction, value, min, max, invert, onChange }: ResizeHandleProps) {
  const onMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      const startPos = direction === "horizontal" ? e.clientX : e.clientY;
      const startValue = value;
      const cursor = direction === "horizontal" ? "col-resize" : "row-resize";

      const overlay = document.createElement("div");
      overlay.style.cssText = `position:fixed;inset:0;z-index:9999;cursor:${cursor};`;
      document.body.appendChild(overlay);
      document.body.style.cursor = cursor;
      document.body.style.userSelect = "none";

      const onMouseMove = (ev: MouseEvent) => {
        const currentPos = direction === "horizontal" ? ev.clientX : ev.clientY;
        const delta = currentPos - startPos;
        const newValue = startValue + (invert ? -delta : delta);
        onChange(Math.max(min, Math.min(max, newValue)));
      };

      const onMouseUp = () => {
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
        overlay.remove();
        document.removeEventListener("mousemove", onMouseMove);
        document.removeEventListener("mouseup", onMouseUp);
      };

      document.addEventListener("mousemove", onMouseMove);
      document.addEventListener("mouseup", onMouseUp);
    },
    [direction, value, min, max, invert, onChange]
  );

  return (
    <div
      className={`resize-handle resize-handle-${direction}`}
      onMouseDown={onMouseDown}
    />
  );
}
