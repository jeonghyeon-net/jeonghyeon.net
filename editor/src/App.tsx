import { useState, useEffect, useCallback, useRef } from "react";
import { invoke, convertFileSrc } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { getCurrentWindow } from "@tauri-apps/api/window";
import "./App.css";
import FileTree from "./components/FileTree";
import ResizeHandle from "./components/ResizeHandle";
import Terminal from "./components/Terminal";
import Editor from "./components/Editor";
import Preview from "./components/Preview";

function App() {
  const [projectPath, setProjectPath] = useState<string | null>(null);
  const [currentFile, setCurrentFile] = useState<string | null>(null);
  const [content, setContent] = useState<string>("");
  const [isDirty, setIsDirty] = useState(false);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [insertText, setInsertText] = useState<string | null>(null);
  const [renderTrigger, setRenderTrigger] = useState(0);
  const [terminalVisible, setTerminalVisible] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingStatus, setLoadingStatus] = useState("Initializing...");

  // Layout sizes
  const [sidebarWidth, setSidebarWidth] = useState(220);
  const [terminalHeight, setTerminalHeight] = useState(260);
  const mainRef = useRef<HTMLDivElement>(null);

  // Startup flow
  useEffect(() => {
    const startup = async () => {
      try {
        setLoadingStatus("Resolving repository path...");
        const repoPath = await invoke<string>("get_repo_path");
        setProjectPath(repoPath);

        setLoadingStatus("Checking repository...");
        const cloned = await invoke<boolean>("ensure_repo_cloned", { repoPath });
        if (cloned) {
          setLoadingStatus("Repository cloned.");
        }

        setLoadingStatus("Checking git hooks...");
        const hooksConfigured = await invoke<boolean>("check_hooks_configured", {
          projectPath: repoPath,
        });
        if (!hooksConfigured) {
          setLoadingStatus("Setting up git hooks...");
          await invoke("setup_hooks", { projectPath: repoPath });
        }

        setLoadingStatus("Building transformer...");
        await invoke("ensure_transformer_built", { projectPath: repoPath });

        setLoadingStatus("Starting file watcher...");
        await invoke("watch_content_dir", { projectPath: repoPath });

        setLoading(false);
      } catch (e) {
        console.error("Startup failed:", e);
        setLoadingStatus(`Error: ${e}`);
      }
    };

    startup();
  }, []);

  // Save handler
  const handleSave = useCallback(async () => {
    if (!currentFile) return;
    lastSaveTimeRef.current = Date.now();
    await invoke("write_file", { path: currentFile, content });
    setIsDirty(false);
    setRenderTrigger((prev) => prev + 1);
  }, [currentFile, content]);

  // Autosave (1 second debounce)
  useEffect(() => {
    if (!isDirty || !currentFile) return;
    const timer = setTimeout(async () => {
      lastSaveTimeRef.current = Date.now();
      await invoke("write_file", { path: currentFile, content });
      setIsDirty(false);
      setRenderTrigger((prev) => prev + 1);
    }, 1000);
    return () => clearTimeout(timer);
  }, [isDirty, content, currentFile]);

  // Global keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.metaKey && e.key === "s" && !e.ctrlKey && !e.altKey && !e.shiftKey) {
        e.preventDefault();
        handleSave();
      }
      if (e.metaKey && e.key === "j" && !e.ctrlKey && !e.altKey && !e.shiftKey) {
        e.preventDefault();
        setTerminalVisible((v) => !v);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [handleSave]);

  const [terminalCwds, setTerminalCwds] = useState<string[]>([]);
  const [activeTerminalCwd, setActiveTerminalCwd] = useState<string | null>(null);

  // Reload current file when changed externally
  const currentFileRef = useRef(currentFile);
  currentFileRef.current = currentFile;
  const lastSaveTimeRef = useRef(0);

  const isDirtyRef = useRef(isDirty);
  isDirtyRef.current = isDirty;

  useEffect(() => {
    let unlisten: (() => void) | undefined;
    listen<string[]>("content-changed", async (event) => {
      const file = currentFileRef.current;
      if (!file) return;
      // Only reload if the changed paths include the current file
      const changedPaths = event.payload;
      if (!changedPaths?.some((p) => p === file)) return;
      // Skip if we just saved (avoid reloading our own writes)
      if (Date.now() - lastSaveTimeRef.current < 1000) return;
      try {
        const newContent = await invoke<string>("read_file", { path: file });
        setContent(newContent);
        setRenderTrigger((prev) => prev + 1);
      } catch {
        // File might have been deleted
      }
    }).then((fn) => { unlisten = fn; });
    return () => { unlisten?.(); };
  }, []);

  // Image viewer state
  const [viewingImage, setViewingImage] = useState<string | null>(null);

  // File selection
  const handleFileSelect = useCallback(async (path: string) => {
    const ext = path.substring(path.lastIndexOf(".")).toLowerCase();
    const imageExts = [".webp", ".png", ".jpg", ".jpeg", ".gif", ".bmp"];

    const dir = path.substring(0, path.lastIndexOf("/"));
    setActiveTerminalCwd(dir);
    setTerminalCwds((prev) => prev.includes(dir) ? prev : [...prev, dir]);
    setTerminalVisible(true);

    if (imageExts.includes(ext)) {
      setViewingImage(path);
      setSelectedFile(path);
      setCurrentFile(null);
      setContent("");
      return;
    }

    setViewingImage(null);
    setSelectedFile(path);
    try {
      const fileContent = await invoke<string>("read_file", { path });
      setCurrentFile(path);
      setContent(fileContent);
      setIsDirty(false);
    } catch (e) {
      console.error("Failed to read file:", e);
    }
  }, []);

  // Image drop handler
  const handleImageDrop = useCallback(
    async (paths: string[]) => {
      if (!currentFile) return;
      const destDir = currentFile.substring(0, currentFile.lastIndexOf("/"));
      const imageExts = [".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tiff"];

      for (const sourcePath of paths) {
        const ext = sourcePath.substring(sourcePath.lastIndexOf(".")).toLowerCase();
        if (!imageExts.includes(ext)) continue;

        try {
          const webpFilename = await invoke<string>("optimize_image", {
            sourcePath,
            destDir,
          });
          setInsertText(`![](${webpFilename})\n`);
          await new Promise((r) => setTimeout(r, 0));
          setInsertText(null);
        } catch (e) {
          console.error("Failed to optimize image:", e);
        }
      }
    },
    [currentFile]
  );

  // New post creation — parentPath is where the folder goes
  const handleNewPost = useCallback(
    async (parentPath: string, slug: string) => {
      const title = slug.replace(/^\d{2}-/, "").split("-").map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(" ");
      const filePath = `${parentPath}/${slug}/index.md`;
      try {
        await invoke("write_file", { path: filePath, content: `# ${title}\n\n` });
        handleFileSelect(filePath);
      } catch (e) {
        console.error("Failed to create post:", e);
      }
    },
    [handleFileSelect]
  );

  // Rename file/folder
  const handleRename = useCallback(
    async (oldPath: string, newName: string) => {
      const parentDir = oldPath.substring(0, oldPath.lastIndexOf("/"));
      const newPath = `${parentDir}/${newName}`;
      if (oldPath === newPath) return;
      try {
        await invoke("rename_path", { oldPath, newPath });
        if (currentFile?.startsWith(oldPath)) {
          handleFileSelect(currentFile.replace(oldPath, newPath));
        }
      } catch (e) {
        console.error("Failed to rename:", e);
      }
    },
    [currentFile, handleFileSelect]
  );

  if (loading) {
    return (
      <div className="loading-screen">
        <div className="loading-spinner" />
        <div className="loading-title">jeonghyeon.net editor</div>
        <div className="loading-status">{loadingStatus}</div>
      </div>
    );
  }

  const fileName = currentFile ? currentFile.split("/").pop() : null;

  return (
    <>
      <div
        className={`app ${terminalVisible ? "" : "terminal-hidden"}`}
        style={{
          "--sidebar-width": `${sidebarWidth}px`,
          "--terminal-height": `${terminalHeight}px`,
        } as React.CSSProperties}
      >
        {/* Title Bar */}
        <div className="titlebar" data-tauri-drag-region>
          <span className="titlebar-text" data-tauri-drag-region>
            {fileName ? `${fileName} - jeonghyeon.net editor` : "jeonghyeon.net editor"}
          </span>
          <div className="titlebar-buttons">
            <button className="titlebar-btn" onClick={() => getCurrentWindow().minimize()}><span className="win-icon win-minimize" /></button>
            <button className="titlebar-btn" onClick={() => getCurrentWindow().toggleMaximize()}><span className="win-icon win-maximize" /></button>
            <button className="titlebar-btn" onClick={() => getCurrentWindow().close()}><span className="win-icon win-close" /></button>
          </div>
        </div>


        {/* Sidebar */}
        <div className="sidebar">
          {projectPath && (
            <FileTree
              projectPath={projectPath}
              currentFile={selectedFile}
              isDirty={isDirty}
              onFileSelect={handleFileSelect}
              onNewPost={handleNewPost}
              onRename={handleRename}
            />
          )}
        </div>

        {/* Sidebar resize */}
        <div className="sidebar-resize">
          <ResizeHandle
            direction="horizontal"
            value={sidebarWidth}
            min={120}
            max={500}
            onChange={setSidebarWidth}
          />
        </div>

        {/* Editor + Preview */}
        <div className="main-content" ref={mainRef}>
          {viewingImage ? (
            <div className="image-viewer">
              <div className="pane-header">Preview</div>
              <div className="image-viewer-content">
                <img src={convertFileSrc(viewingImage)} alt={viewingImage.split("/").pop() || ""} />
              </div>
            </div>
          ) : currentFile?.endsWith(".md") ? (
            <>
              <div className="editor-pane">
                <div className="pane-header">Editor</div>
                <Editor
                  filePath={currentFile}
                  content={content}
                  onContentChange={(newContent) => {
                    setContent(newContent);
                    setIsDirty(true);
                  }}
                  onSave={handleSave}
                  insertText={insertText}
                  onImageDrop={handleImageDrop}
                />
              </div>
              <div className="pane-divider" />
              <div className="preview-pane" style={{ width: 740, flex: "none" }}>
                <div className="pane-header">Preview</div>
                {projectPath && (
                  <Preview
                    projectPath={projectPath}
                    filePath={currentFile}
                    triggerRender={renderTrigger}
                  />
                )}
              </div>
            </>
          ) : (
            <div className="editor-pane" style={{ flex: 1 }}>
              <div className="pane-header">Editor</div>
              <Editor
                filePath={currentFile}
                content={content}
                onContentChange={(newContent) => {
                  setContent(newContent);
                  setIsDirty(true);
                }}
                onSave={handleSave}
                insertText={insertText}
                onImageDrop={handleImageDrop}
              />
            </div>
          )}
        </div>

        {/* Terminal resize */}
        {terminalVisible && (
          <div className="terminal-resize">
            <ResizeHandle
              direction="vertical"
              value={terminalHeight}
              min={80}
              max={600}
              invert
              onChange={setTerminalHeight}
            />
          </div>
        )}

        {/* Terminal */}
        <div className="terminal-area">
          <div className="pane-header">Terminal</div>
          {terminalCwds.map((cwd) => (
            <Terminal key={cwd} cwd={cwd} visible={terminalVisible && cwd === activeTerminalCwd} />
          ))}
        </div>

        {/* Status Bar */}
        <div className="statusbar">
          <span className="statusbar-section">
            {currentFile ? currentFile.replace(projectPath + "/content/", "") : "Ready"}
          </span>
          <span className="statusbar-section">{isDirty ? "Modified" : "Saved"}</span>
          <span className="statusbar-section">{terminalVisible ? "Terminal: ON" : "Terminal: OFF"}</span>
        </div>
      </div>

    </>
  );
}

export default App;
