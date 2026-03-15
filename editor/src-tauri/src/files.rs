use notify::{EventKind, RecursiveMode, Watcher};
use std::path::Path;
use tauri::{AppHandle, Emitter, Manager};

use crate::util::is_safe_path;

pub struct WatcherState(pub std::sync::Mutex<Option<notify::RecommendedWatcher>>);

#[derive(serde::Serialize, Clone)]
pub struct FileEntry {
    pub name: String,
    pub path: String,
    pub is_dir: bool,
    pub children: Vec<FileEntry>,
}

fn build_tree(dir: &Path) -> std::io::Result<Vec<FileEntry>> {
    let mut entries = Vec::new();
    for entry in std::fs::read_dir(dir)? {
        let entry = entry?;
        let path = entry.path();
        let name = entry.file_name().to_string_lossy().to_string();
        let is_dir = path.is_dir();
        let children = if is_dir {
            build_tree(&path).unwrap_or_default()
        } else {
            vec![]
        };
        entries.push(FileEntry {
            name,
            path: path.to_string_lossy().to_string(),
            is_dir,
            children,
        });
    }
    entries.sort_by(|a, b| b.is_dir.cmp(&a.is_dir).then(a.name.cmp(&b.name)));
    Ok(entries)
}

#[tauri::command]
pub async fn read_file(path: String) -> Result<String, String> {
    if !is_safe_path(&path) {
        return Err("Invalid file path".to_string());
    }
    tauri::async_runtime::spawn_blocking(move || {
        std::fs::read_to_string(&path).map_err(|e| format!("Failed to read file: {}", e))
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))?
}

#[tauri::command]
pub async fn write_file(path: String, content: String) -> Result<(), String> {
    if !is_safe_path(&path) {
        return Err("Invalid file path".to_string());
    }
    tauri::async_runtime::spawn_blocking(move || {
        let p = Path::new(&path);
        if let Some(parent) = p.parent() {
            std::fs::create_dir_all(parent)
                .map_err(|e| format!("Failed to create directories: {}", e))?;
        }
        std::fs::write(p, content).map_err(|e| format!("Failed to write file: {}", e))
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))?
}

#[tauri::command]
pub async fn rename_path(old_path: String, new_path: String) -> Result<(), String> {
    if !is_safe_path(&old_path) || !is_safe_path(&new_path) {
        return Err("Invalid file path".to_string());
    }
    tauri::async_runtime::spawn_blocking(move || {
        std::fs::rename(&old_path, &new_path)
            .map_err(|e| format!("Failed to rename: {}", e))
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))?
}

#[tauri::command]
pub async fn list_content_tree(project_path: String) -> Result<Vec<FileEntry>, String> {
    if !is_safe_path(&project_path) {
        return Err("Invalid project path".to_string());
    }
    tauri::async_runtime::spawn_blocking(move || {
        let content_dir = Path::new(&project_path).join("content");
        build_tree(&content_dir).map_err(|e| format!("Failed to list content tree: {}", e))
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))?
}

#[tauri::command]
pub async fn watch_content_dir(app: AppHandle, project_path: String) -> Result<(), String> {
    if !is_safe_path(&project_path) {
        return Err("Invalid project path".to_string());
    }
    let content_dir = Path::new(&project_path).join("content");
    if !content_dir.exists() {
        return Err("Content directory does not exist".to_string());
    }

    let content_path = content_dir.to_path_buf();
    let app_clone = app.clone();
    let mut watcher =
        notify::recommended_watcher(move |res: Result<notify::Event, notify::Error>| {
            if let Ok(event) = res {
                match event.kind {
                    EventKind::Create(_) | EventKind::Modify(_) | EventKind::Remove(_) => {
                        let _ = app_clone.emit("content-changed", &event.paths);
                    }
                    _ => {}
                }
            }
        })
        .map_err(|e| format!("Failed to create file watcher: {}", e))?;

    watcher
        .watch(&content_path, RecursiveMode::Recursive)
        .map_err(|e| format!("Failed to watch content directory: {}", e))?;

    // Store the watcher in managed state so it stays alive and can be replaced
    let state = app.state::<WatcherState>();
    let mut guard = state.0.lock().map_err(|e| format!("Lock error: {}", e))?;
    *guard = Some(watcher);

    Ok(())
}
