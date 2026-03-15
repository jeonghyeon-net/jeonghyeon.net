import { useEffect, useRef } from "react";
import { EditorState } from "@codemirror/state";
import { EditorView, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { markdown } from "@codemirror/lang-markdown";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { tags } from "@lezer/highlight";
import { getCurrentWindow } from "@tauri-apps/api/window";

interface EditorProps {
  filePath: string | null;
  content: string;
  onContentChange: (content: string) => void;
  onSave: () => void;
  insertText: string | null;
  onImageDrop?: (paths: string[]) => void;
}

const win98Theme = EditorView.theme({
  "&": {
    height: "100%",
    backgroundColor: "#ffffff",
    color: "#000000",
    fontFamily: '"Courier New", Courier, monospace',
    fontSize: "13px",
  },
  ".cm-scroller": { overflow: "auto" },
  ".cm-gutters": {
    backgroundColor: "#c0c0c0",
    color: "#000000",
    border: "none",
    borderRight: "2px solid",
    borderRightColor: "#808080",
  },
  ".cm-activeLineGutter": { backgroundColor: "#a0a0a0" },
  ".cm-activeLine": { backgroundColor: "#e8e8e8" },
  ".cm-selectionBackground, &.cm-focused .cm-selectionBackground": {
    backgroundColor: "#000080 !important",
  },
  ".cm-cursor": { borderLeftColor: "#000000" },
  "&.cm-focused .cm-selectionBackground .cm-selectionMatch": {
    backgroundColor: "#000080",
  },
  ".cm-line ::selection": { backgroundColor: "#000080", color: "#ffffff" },
  ".cm-selectionMatch": { backgroundColor: "#d0d0ff" },
});

const win98Highlight = HighlightStyle.define([
  { tag: tags.heading1, fontWeight: "bold", color: "#000080" },
  { tag: tags.heading2, fontWeight: "bold", color: "#000080" },
  { tag: tags.heading3, fontWeight: "bold", color: "#000080" },
  { tag: tags.heading, fontWeight: "bold", color: "#000080" },
  { tag: tags.strong, fontWeight: "bold" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.link, color: "#0000ff", textDecoration: "underline" },
  { tag: tags.url, color: "#0000ff" },
  { tag: tags.monospace, color: "#008000", fontFamily: '"Courier New", monospace' },
  { tag: tags.processingInstruction, color: "#808080" }, // markdown markers like **, ##
  { tag: tags.comment, color: "#808080" },
  { tag: tags.meta, color: "#808080" },
  { tag: tags.quote, color: "#808000" },
  { tag: tags.list, color: "#800000" },
]);

function Editor({
  filePath,
  content,
  onContentChange,
  onSave,
  insertText,
  onImageDrop,
}: EditorProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const onContentChangeRef = useRef(onContentChange);
  const onSaveRef = useRef(onSave);

  // Keep refs in sync
  onContentChangeRef.current = onContentChange;
  onSaveRef.current = onSave;

  // Create/destroy editor when filePath changes
  useEffect(() => {
    if (!containerRef.current || !filePath) return;

    const state = EditorState.create({
      doc: content,
      extensions: [
        history(),
        keymap.of([
          ...defaultKeymap,
          ...historyKeymap,
          {
            key: "Mod-s",
            run: () => {
              onSaveRef.current();
              return true;
            },
          },
        ]),
        lineNumbers({ formatNumber: (n) => String(n).padStart(4, "\u00a0") }),
        markdown(),
        win98Theme,
        syntaxHighlighting(win98Highlight),
        EditorView.lineWrapping,
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            onContentChangeRef.current(update.state.doc.toString());
          }
        }),
      ],
    });

    const view = new EditorView({
      state,
      parent: containerRef.current,
    });

    viewRef.current = view;

    return () => {
      view.destroy();
      viewRef.current = null;
    };
    // Only re-create when filePath changes — content is the initial value
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filePath]);

  // Sync external content changes into the editor
  const contentRef = useRef(content);
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const currentDoc = view.state.doc.toString();
    if (content !== currentDoc && content !== contentRef.current) {
      view.dispatch({
        changes: { from: 0, to: currentDoc.length, insert: content },
      });
    }
    contentRef.current = content;
  }, [content]);

  // Insert text at cursor
  useEffect(() => {
    if (!insertText || !viewRef.current) return;

    const view = viewRef.current;
    const cursor = view.state.selection.main.head;
    view.dispatch({
      changes: { from: cursor, insert: insertText },
      selection: { anchor: cursor + insertText.length },
    });
  }, [insertText]);

  // Tauri drag-drop listener
  useEffect(() => {
    if (!filePath || !onImageDrop) return;

    let unlisten: (() => void) | undefined;

    getCurrentWindow()
      .onDragDropEvent((event) => {
        if (event.payload.type === "drop") {
          onImageDrop(event.payload.paths);
        }
      })
      .then((fn) => {
        unlisten = fn;
      });

    return () => {
      unlisten?.();
    };
  }, [filePath, onImageDrop]);

  if (!filePath) {
    return <div className="editor-empty">Select a file to edit</div>;
  }

  return <div className="editor-container" ref={containerRef} />;
}

export default Editor;
