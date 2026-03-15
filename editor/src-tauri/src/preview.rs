use std::path::{Path, PathBuf};
use std::process::Command;
use tauri::AppHandle;
#[cfg(not(dev))]
use tauri::Manager;

fn extended_path() -> String {
    let current_path = std::env::var("PATH").unwrap_or_default();
    format!(
        "/opt/homebrew/bin:/usr/local/bin:/usr/local/go/bin:{}",
        current_path
    )
}

fn get_transformer_path(app: &AppHandle, project_path: &str) -> PathBuf {
    // Dev mode: use the transformer from the repo
    #[cfg(dev)]
    {
        let _ = app;
        return Path::new(project_path).join("transformer").join("transformer");
    }

    // Production: use the bundled sidecar
    #[cfg(not(dev))]
    {
        let _ = project_path;
        let resource_dir = app.path().resource_dir()
            .expect("Failed to get resource dir");
        resource_dir.join("binaries").join("transformer")
    }
}

#[tauri::command]
pub async fn render_single_file(
    app: AppHandle,
    project_path: String,
    md_path: String,
) -> Result<String, String> {
    let transformer_bin = get_transformer_path(&app, &project_path);

    tauri::async_runtime::spawn_blocking(move || {
        let content_dir = Path::new(&project_path).join("content");

        let output = Command::new(&transformer_bin)
            .args(["render-single", &content_dir.to_string_lossy(), &md_path])
            .env("PATH", extended_path())
            .output()
            .map_err(|e| format!("Failed to run transformer: {}", e))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!("transformer render-single failed: {}", stderr));
        }

        Ok(String::from_utf8_lossy(&output.stdout).to_string())
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))?
}
