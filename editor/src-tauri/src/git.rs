use std::path::Path;
use std::process::Command;

use crate::util::extended_path;

const REPO_URL: &str = "git@github.com:jeonghyeon-net/jeonghyeon.net.git";

#[tauri::command]
pub async fn get_repo_path() -> Result<String, String> {
    // Dev mode: editor/ is inside jeonghyeon.net/, so repo is ../
    #[cfg(dev)]
    {
        let manifest_dir = std::path::Path::new(env!("CARGO_MANIFEST_DIR")); // src-tauri/
        let repo_dir = manifest_dir
            .parent() // editor/
            .and_then(|p| p.parent()) // jeonghyeon.net/
            .ok_or("Could not determine repo path")?;
        return Ok(repo_dir.to_string_lossy().to_string());
    }

    // Production: clone to Application Support
    #[cfg(not(dev))]
    {
        let dir = dirs::data_dir()
            .ok_or_else(|| "Could not determine application support directory".to_string())?;
        let repo_path = dir.join("net.jeonghyeon.editor").join("repo");
        Ok(repo_path.to_string_lossy().to_string())
    }
}

#[tauri::command]
pub async fn ensure_repo_cloned(repo_path: String) -> Result<bool, String> {
    tauri::async_runtime::spawn_blocking(move || {
        let path = Path::new(&repo_path);
        let git_dir = path.join(".git");
        let content_dir = path.join("content");

        // If .git exists but content/ doesn't, previous clone was incomplete
        if git_dir.exists() && !content_dir.exists() {
            let _ = std::fs::remove_dir_all(path);
        }

        if git_dir.exists() && content_dir.exists() {
            return Ok(false);
        }

        std::fs::create_dir_all(path)
            .map_err(|e| format!("Failed to create repo directory: {}", e))?;

        let output = Command::new("git")
            .args(["clone", REPO_URL, "."])
            .current_dir(path)
            .env("PATH", extended_path())
            .output()
            .map_err(|e| format!("Failed to run git clone: {}", e))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!("git clone failed: {}", stderr));
        }

        Ok(true)
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))?
}

#[tauri::command]
pub async fn check_hooks_configured(project_path: String) -> Result<bool, String> {
    tauri::async_runtime::spawn_blocking(move || {
        let output = Command::new("git")
            .args(["config", "core.hooksPath"])
            .current_dir(&project_path)
            .env("PATH", extended_path())
            .output()
            .map_err(|e| format!("Failed to run git config: {}", e))?;

        let value = String::from_utf8_lossy(&output.stdout).trim().to_string();
        Ok(value == "hooks")
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))?
}

#[tauri::command]
pub async fn setup_hooks(project_path: String) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || {
        let output = Command::new("git")
            .args(["config", "core.hooksPath", "hooks"])
            .current_dir(&project_path)
            .env("PATH", extended_path())
            .output()
            .map_err(|e| format!("Failed to run git config: {}", e))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!("Failed to set hooks path: {}", stderr));
        }

        Ok(())
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))?
}

#[tauri::command]
pub async fn ensure_transformer_built(project_path: String) -> Result<(), String> {
    // Production: transformer is bundled as sidecar, nothing to build
    #[cfg(not(dev))]
    {
        let _ = project_path;
        return Ok(());
    }

    // Dev mode: build from source if missing
    #[cfg(dev)]
    tauri::async_runtime::spawn_blocking(move || {
        let transformer_bin = Path::new(&project_path)
            .join("transformer")
            .join("transformer");

        if transformer_bin.exists() {
            return Ok(());
        }

        let transformer_dir = Path::new(&project_path).join("transformer");

        let output = Command::new("go")
            .args(["build", "-o", "transformer", "."])
            .current_dir(&transformer_dir)
            .env("PATH", extended_path())
            .output()
            .map_err(|e| format!("Failed to run go build: {}", e))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!("go build failed: {}", stderr));
        }

        Ok(())
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))?
}
