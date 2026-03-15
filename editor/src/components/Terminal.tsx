import { useEffect, useRef, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import { WebLinksAddon } from "@xterm/addon-web-links";
import "@xterm/xterm/css/xterm.css";

type TerminalProps = {
  cwd: string;
  visible: boolean;
};

export default function Terminal({ cwd, visible }: TerminalProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<XTerm | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const sessionIdRef = useRef<number | null>(null);
  const refit = useCallback(() => {
    if (fitAddonRef.current && termRef.current && containerRef.current) {
      const { offsetWidth, offsetHeight } = containerRef.current;
      if (offsetWidth > 0 && offsetHeight > 0) {
        try {
          fitAddonRef.current.fit();
          if (sessionIdRef.current !== null) {
            invoke("resize_pty", {
              sessionId: sessionIdRef.current,
              rows: termRef.current.rows,
              cols: termRef.current.cols,
            }).catch(() => {});
          }
        } catch {
          // fit() can throw if container is not visible
        }
      }
    }
  }, []);

  // Refit when visibility changes
  useEffect(() => {
    if (visible) {
      requestAnimationFrame(refit);
    }
  }, [visible, refit]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const term = new XTerm({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: '"Courier New", Courier, monospace',
      theme: {
        background: "#000000",
        foreground: "#e0e0e0",
        cursor: "#c0c0c0",
        selectionBackground: "#c0c0c0",
        selectionForeground: "#000000",
        black: "#000000",
        brightBlack: "#666666",
        white: "#c0c0c0",
        brightWhite: "#ffffff",
        blue: "#6c9eff",
        brightBlue: "#9bbfff",
        cyan: "#2ee2e2",
        brightCyan: "#6ff5f5",
        green: "#4ec94e",
        brightGreen: "#7aff7a",
        magenta: "#d07ed0",
        brightMagenta: "#ff9cff",
        red: "#ff6b6b",
        brightRed: "#ff9b9b",
        yellow: "#e5e550",
        brightYellow: "#ffff80",
      },
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.loadAddon(new WebLinksAddon((_event, uri) => { window.open(uri, "_blank"); }));
    term.open(container);

    try { term.loadAddon(new WebglAddon()); } catch { /* canvas fallback */ }

    termRef.current = term;
    fitAddonRef.current = fitAddon;

    // Custom key handler
    term.attachCustomKeyEventHandler((e) => {
      if (e.type === "keydown") {
        // Cmd+K clear
        if (e.metaKey && e.key === "k" && !e.ctrlKey && !e.altKey && !e.shiftKey) {
          term.clear();
          return false;
        }
        // Cmd+J bubble up
        if (e.metaKey && e.key === "j" && !e.ctrlKey && !e.altKey && !e.shiftKey) {
          return false;
        }
        // Option+Arrow word nav
        if (e.altKey && !e.ctrlKey && !e.metaKey) {
          if (e.key === "ArrowLeft") {
            if (sessionIdRef.current !== null) invoke("write_to_pty", { sessionId: sessionIdRef.current, data: "\x1bb" }).catch(console.error);
            return false;
          } else if (e.key === "ArrowRight") {
            if (sessionIdRef.current !== null) invoke("write_to_pty", { sessionId: sessionIdRef.current, data: "\x1bf" }).catch(console.error);
            return false;
          } else if (e.key === "Backspace") {
            if (sessionIdRef.current !== null) invoke("write_to_pty", { sessionId: sessionIdRef.current, data: "\x1b\x7f" }).catch(console.error);
            return false;
          }
        }
        // Block printable chars — handled by beforeinput for Korean IME
        if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
          return false;
        }
      }
      return true;
    });

    // PTY session
    let sessionId: number | null = null;
    let unlistenOutput: (() => void) | null = null;
    let unlistenEnd: (() => void) | null = null;

    const setup = async () => {
      try {
        sessionId = await invoke<number>("create_pty_session", {
          rows: term.rows || 24,
          cols: term.cols || 80,
          cwd,
        });
        sessionIdRef.current = sessionId;

        requestAnimationFrame(() => {
          fitAddon.fit();
          if (sessionId !== null) {
            invoke("resize_pty", { sessionId, rows: term.rows, cols: term.cols }).catch(() => {});
          }
        });

        // Korean IME via beforeinput
        const xtermTextarea = container.querySelector(".xterm-helper-textarea") as HTMLTextAreaElement | null;

        if (xtermTextarea) {
          let isComposing = false;
          let composingText = "";

          const isKorean = (ch: string) => {
            if (!ch) return false;
            const code = ch.charCodeAt(0);
            return (code >= 0x1100 && code <= 0x11ff) || (code >= 0x3130 && code <= 0x318f) || (code >= 0xac00 && code <= 0xd7af);
          };

          xtermTextarea.addEventListener("beforeinput", (e: InputEvent) => {
            const data = e.data || "";
            const inputType = e.inputType;
            const sid = sessionIdRef.current;
            if (sid === null) return;

            if (inputType === "insertFromComposition") {
              e.preventDefault();
              if (data) invoke("write_to_pty", { sessionId: sid, data }).catch(console.error);
              xtermTextarea.dispatchEvent(new CompositionEvent("compositionend", { data: "" }));
              isComposing = false;
              composingText = "";
              return;
            }

            if (inputType === "insertReplacementText" || inputType === "insertCompositionText" || (inputType === "insertText" && isKorean(data))) {
              e.preventDefault();
              if (!isComposing) {
                isComposing = true;
                xtermTextarea.dispatchEvent(new CompositionEvent("compositionstart", { data: "" }));
              }
              composingText = data;
              xtermTextarea.dispatchEvent(new CompositionEvent("compositionupdate", { data }));
              return;
            }

            if (inputType === "insertText") {
              e.preventDefault();
              if (isComposing) {
                if (composingText) invoke("write_to_pty", { sessionId: sid, data: composingText }).catch(console.error);
                xtermTextarea.dispatchEvent(new CompositionEvent("compositionend", { data: "" }));
                isComposing = false;
                composingText = "";
              }
              if (data) invoke("write_to_pty", { sessionId: sid, data }).catch(console.error);
              return;
            }
          });
        }

        // onData — control chars only (printable ASCII handled by beforeinput)
        term.onData((data) => {
          const sid = sessionIdRef.current;
          if (sid === null) return;
          if (data.length === 1 && data.charCodeAt(0) >= 32 && data.charCodeAt(0) !== 127) {
            return; // Skip printable ASCII — beforeinput handles it
          }
          invoke("write_to_pty", { sessionId: sid, data }).catch(console.error);
        });

        unlistenOutput = await listen<string>(`pty-output-${sessionId}`, (event) => {
          term.write(event.payload);
        });

        unlistenEnd = await listen(`pty-end-${sessionId}`, () => {
          term.write("\r\n[Process exited]\r\n");
        });
      } catch (e) {
        console.error("Failed to create PTY session:", e);
        term.write(`\r\nFailed to create terminal: ${e}\r\n`);
      }
    };

    setup();

    const observer = new ResizeObserver(() => {
      {
        fitAddon.fit();
        const sid = sessionIdRef.current;
        if (sid !== null) {
          invoke("resize_pty", { sessionId: sid, rows: term.rows, cols: term.cols }).catch(() => {});
        }
      }
    });
    observer.observe(container);

    return () => {
      observer.disconnect();
      unlistenOutput?.();
      unlistenEnd?.();
      const sid = sessionIdRef.current;
      if (sid !== null) {
        invoke("close_pty_session", { sessionId: sid }).catch(() => {});
        sessionIdRef.current = null;
      }
      term.dispose();
      termRef.current = null;
      fitAddonRef.current = null;
    };
  }, [cwd]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div
      className="terminal-container"
      ref={containerRef}
      style={{ display: visible ? "block" : "none" }}
      onClick={() => termRef.current?.focus()}
    />
  );
}
