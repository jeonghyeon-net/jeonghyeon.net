import { useState, useEffect, useCallback, useRef } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";

type FileEntry = {
  name: string;
  path: string;
  is_dir: boolean;
  children: FileEntry[];
};

type FileTreeProps = {
  projectPath: string;
  currentFile: string | null;
  isDirty: boolean;
  onFileSelect: (path: string) => void;
  onNewPost: (parentPath: string, slug: string) => void;
  onRename: (oldPath: string, newName: string) => void;
};

// Detect if a directory is a series (has children with 2-digit prefixes)
function isSeries(entry: FileEntry): boolean {
  return entry.is_dir && entry.children.some(
    (c) => c.is_dir && /^\d{2}-/.test(c.name)
  );
}

// Get the next series number
function nextSeriesNumber(entry: FileEntry): string {
  const nums = entry.children
    .filter((c) => c.is_dir && /^\d{2}-/.test(c.name))
    .map((c) => parseInt(c.name.slice(0, 2), 10));
  const next = nums.length > 0 ? Math.max(...nums) + 1 : 1;
  return String(next).padStart(2, "0");
}

// Inline input for creating/renaming
function InlineInput({
  depth,
  icon,
  defaultValue,
  placeholder,
  onConfirm,
  onCancel,
}: {
  depth: number;
  icon: string;
  defaultValue?: string;
  placeholder?: string;
  onConfirm: (value: string) => void;
  onCancel: () => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [value, setValue] = useState(defaultValue ?? "");

  useEffect(() => {
    const input = inputRef.current;
    if (input) {
      input.focus();
      if (defaultValue) input.select();
    }
  }, []);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      const trimmed = value.trim();
      if (trimmed && trimmed !== defaultValue) onConfirm(trimmed);
      else onCancel();
    } else if (e.key === "Escape") {
      onCancel();
    }
  };

  return (
    <div className="tree-item file" style={{ paddingLeft: depth * 16 + 8 }}>
      <span className={`tree-icon ${icon}`} />
      <input
        ref={inputRef}
        className="inline-rename-input"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={handleKeyDown}
        onBlur={onCancel}
        placeholder={placeholder}
      />
    </div>
  );
}

type ContextMenuInfo = {
  x: number;
  y: number;
  entry: FileEntry | null; // null = background area
};

type InlineAction =
  | { type: "new-post"; parentPath: string; prefix: string }
  | { type: "rename"; entry: FileEntry };

function TreeNode({
  entry,
  currentFile,
  isDirty,
  onFileSelect,
  onContextMenu,
  depth,
  inlineAction,
  onInlineConfirm,
  onInlineCancel,
}: {
  entry: FileEntry;
  currentFile: string | null;
  isDirty: boolean;
  onFileSelect: (path: string) => void;
  onContextMenu: (e: React.MouseEvent, entry: FileEntry) => void;
  depth: number;
  inlineAction: InlineAction | null;
  onInlineConfirm: (value: string) => void;
  onInlineCancel: () => void;
}) {
  const [expanded, setExpanded] = useState(depth < 1);

  // Auto-expand when creating inside or when selected file is inside this folder
  const shouldExpand =
    (inlineAction?.type === "new-post" && entry.path === inlineAction.parentPath) ||
    (currentFile && currentFile.startsWith(entry.path + "/"));

  useEffect(() => {
    if (shouldExpand) setExpanded(true);
  }, [shouldExpand]);

  const isActive = currentFile === entry.path;
  const isRenaming = inlineAction?.type === "rename" && inlineAction.entry.path === entry.path;

  if (entry.is_dir) {
    return (
      <div className="tree-dir">
        {isRenaming ? (
          <InlineInput
            depth={depth}
            icon={expanded ? "icon-folder-open" : "icon-folder"}
            defaultValue={entry.name}
            onConfirm={onInlineConfirm}
            onCancel={onInlineCancel}
          />
        ) : (
          <div
            className="tree-item dir"
            style={{ paddingLeft: depth * 16 + 8 }}
            onClick={() => setExpanded((v) => !v)}
            onContextMenu={(e) => { e.stopPropagation(); onContextMenu(e, entry); }}
          >
            <span className={`tree-arrow ${expanded ? "expanded" : ""}`}>
              {expanded ? "\u25BE" : "\u25B8"}
            </span>
            <span className={`tree-icon ${expanded ? "icon-folder-open" : "icon-folder"}`} />
            <span className="tree-name">{entry.name}</span>
          </div>
        )}
        {expanded && (
          <div className="tree-children">
            {entry.children.map((child) => (
              <TreeNode
                key={child.path}
                entry={child}
                currentFile={currentFile}
                isDirty={isDirty}
                onFileSelect={onFileSelect}
                onContextMenu={onContextMenu}
                depth={depth + 1}
                inlineAction={inlineAction}
                onInlineConfirm={onInlineConfirm}
                onInlineCancel={onInlineCancel}
              />
            ))}
            {/* New post/entry input */}
            {inlineAction?.type === "new-post" && entry.path === inlineAction.parentPath && (
              <InlineInput
                depth={depth + 1}
                icon="icon-folder"
                defaultValue={inlineAction.prefix}
                placeholder={inlineAction.prefix ? inlineAction.prefix + "entry-name" : "post-slug"}
                onConfirm={onInlineConfirm}
                onCancel={onInlineCancel}
              />
            )}
          </div>
        )}
      </div>
    );
  }

  if (isRenaming) {
    return (
      <InlineInput
        depth={depth}
        icon="icon-file"
        defaultValue={entry.name}
        onConfirm={onInlineConfirm}
        onCancel={onInlineCancel}
      />
    );
  }

  return (
    <div
      className={`tree-item file ${isActive ? "active" : ""}`}
      style={{ paddingLeft: depth * 16 + 8 }}
      onClick={() => onFileSelect(entry.path)}
      onContextMenu={(e) => { e.stopPropagation(); onContextMenu(e, entry); }}
    >
      <span className="tree-icon icon-file" />
      <span className="tree-name">{entry.name}</span>
      {isActive && isDirty && <span className="dirty-dot" />}
    </div>
  );
}

export default function FileTree({
  projectPath,
  currentFile,
  isDirty,
  onFileSelect,
  onNewPost,
  onRename,
}: FileTreeProps) {
  const [tree, setTree] = useState<FileEntry[]>([]);
  const [inlineAction, setInlineAction] = useState<InlineAction | null>(null);
  const [contextMenu, setContextMenu] = useState<ContextMenuInfo | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const loadTree = useCallback(async () => {
    try {
      const entries = await invoke<FileEntry[]>("list_content_tree", { projectPath });
      setTree(entries);
    } catch (e) {
      console.error("Failed to load content tree:", e);
    }
  }, [projectPath]);

  useEffect(() => {
    loadTree();
    let unlisten: (() => void) | undefined;
    let debounceTimer: ReturnType<typeof setTimeout> | undefined;
    listen("content-changed", () => {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(loadTree, 300);
    }).then((fn) => { unlisten = fn; });
    return () => { unlisten?.(); clearTimeout(debounceTimer); };
  }, [loadTree]);

  // Close context menu on outside click
  useEffect(() => {
    if (!contextMenu) return;
    const close = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setContextMenu(null);
    };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [contextMenu]);

  const handleContextMenu = useCallback((e: React.MouseEvent, entry: FileEntry | null) => {
    e.preventDefault();
    setContextMenu({ x: e.clientX, y: e.clientY, entry });
  }, []);

  // Find entry by path (recursive)
  const findEntry = useCallback((entries: FileEntry[], path: string): FileEntry | null => {
    for (const e of entries) {
      if (e.path === path) return e;
      if (e.is_dir) {
        const found = findEntry(e.children, path);
        if (found) return found;
      }
    }
    return null;
  }, []);

  // Enter/F2 on selected file → rename
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if ((e.key === "Enter" || e.key === "F2") && currentFile && !inlineAction) {
      e.preventDefault();
      const entry = findEntry(tree, currentFile);
      if (entry) {
        setInlineAction({ type: "rename", entry });
      }
    }
  }, [currentFile, tree, inlineAction, findEntry]);

  const handleInlineConfirm = useCallback((value: string) => {
    if (!inlineAction) return;

    if (inlineAction.type === "new-post") {
      const slug = value.trim().toLowerCase().replace(/[^a-z0-9-]/g, "-").replace(/-+/g, "-").replace(/^-|-$/g, "");
      if (slug) onNewPost(inlineAction.parentPath, slug);
    } else if (inlineAction.type === "rename") {
      onRename(inlineAction.entry.path, value.trim());
    }
    setInlineAction(null);
  }, [inlineAction, onNewPost, onRename]);

  // Find the posts directory path
  const postsPath = tree.find((e) => e.name === "posts")?.path;

  // Build context menu items based on target
  const menuItems: { label: string; action: () => void }[] = [];
  if (contextMenu) {
    const entry = contextMenu.entry;

    if (entry && isSeries(entry)) {
      // Right-click on a series folder → new post goes inside as numbered entry
      const num = nextSeriesNumber(entry);
      menuItems.push({
        label: "New Post",
        action: () => {
          setInlineAction({ type: "new-post", parentPath: entry.path, prefix: num + "-" });
          setContextMenu(null);
        },
      });
    } else if (postsPath) {
      // Otherwise → new post at posts root
      menuItems.push({
        label: "New Post",
        action: () => {
          setInlineAction({ type: "new-post", parentPath: postsPath, prefix: "" });
          setContextMenu(null);
        },
      });
    }

    // "Rename" — if right-clicked on a specific item
    if (entry) {
      menuItems.push({
        label: "Rename",
        action: () => {
          setInlineAction({ type: "rename", entry });
          setContextMenu(null);
        },
      });
    }
  }

  return (
    <div className="file-tree" onContextMenu={(e) => handleContextMenu(e, null)} onKeyDown={handleKeyDown} tabIndex={0}>
      <div className="file-tree-header">
        <span className="file-tree-title">Content</span>
      </div>
      <div className="file-tree-list">
        {tree.map((entry) => (
          <TreeNode
            key={entry.path}
            entry={entry}
            currentFile={currentFile}
            isDirty={isDirty}
            onFileSelect={onFileSelect}
            onContextMenu={handleContextMenu}
            depth={0}
            inlineAction={inlineAction}
            onInlineConfirm={handleInlineConfirm}
            onInlineCancel={() => setInlineAction(null)}
          />
        ))}
      </div>

      {contextMenu && (
        <div
          ref={menuRef}
          className="context-menu"
          style={{ top: contextMenu.y, left: contextMenu.x }}
        >
          {menuItems.map((item) => (
            <div key={item.label} className="context-menu-item" onClick={item.action}>
              {item.label}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
