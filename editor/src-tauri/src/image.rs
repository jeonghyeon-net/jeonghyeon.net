use std::path::Path;
use std::process::Command;

const MAX_DIM: u32 = 700;

#[tauri::command]
pub async fn optimize_image(source_path: String, dest_dir: String) -> Result<String, String> {
    tauri::async_runtime::spawn_blocking(move || {
        let src = Path::new(&source_path);
        let dest = Path::new(&dest_dir);

        let file_name = src
            .file_name()
            .ok_or_else(|| "Invalid source path".to_string())?;
        let copied = dest.join(file_name);

        // If already .webp, just copy to destination without conversion
        let src_ext = src
            .extension()
            .map(|e| e.to_string_lossy().to_lowercase())
            .unwrap_or_default();
        if src_ext == "webp" {
            std::fs::copy(src, &copied)
                .map_err(|e| format!("Failed to copy file: {}", e))?;
            return Ok(file_name.to_string_lossy().to_string());
        }

        // 1. Copy source to dest_dir
        std::fs::copy(src, &copied)
            .map_err(|e| format!("Failed to copy file: {}", e))?;

        // 2. Get dimensions via sips
        let width_output = Command::new("sips")
            .args(["-g", "pixelWidth"])
            .arg(&copied)
            .output()
            .map_err(|e| format!("Failed to run sips (width): {}", e))?;
        let width_str = String::from_utf8_lossy(&width_output.stdout);
        let width: u32 = width_str
            .lines()
            .find(|l| l.contains("pixelWidth"))
            .and_then(|l| l.split_whitespace().last())
            .and_then(|v| v.parse().ok())
            .unwrap_or(0);

        let height_output = Command::new("sips")
            .args(["-g", "pixelHeight"])
            .arg(&copied)
            .output()
            .map_err(|e| format!("Failed to run sips (height): {}", e))?;
        let height_str = String::from_utf8_lossy(&height_output.stdout);
        let height: u32 = height_str
            .lines()
            .find(|l| l.contains("pixelHeight"))
            .and_then(|l| l.split_whitespace().last())
            .and_then(|v| v.parse().ok())
            .unwrap_or(0);

        // 3. Build cwebp command
        let stem = src
            .file_stem()
            .ok_or_else(|| "Invalid file name".to_string())?
            .to_string_lossy();
        let webp_name = format!("{}.webp", stem);
        let webp_path = dest.join(&webp_name);

        let mut args: Vec<String> = vec!["-q".to_string(), "80".to_string()];

        let longer_side = width.max(height);
        if longer_side > MAX_DIM {
            if width >= height {
                args.push("-resize".to_string());
                args.push(MAX_DIM.to_string());
                args.push("0".to_string());
            } else {
                args.push("-resize".to_string());
                args.push("0".to_string());
                args.push(MAX_DIM.to_string());
            }
        }

        args.push(copied.to_string_lossy().to_string());
        args.push("-o".to_string());
        args.push(webp_path.to_string_lossy().to_string());

        let output = Command::new("cwebp")
            .args(&args)
            .output()
            .map_err(|e| format!("Failed to run cwebp: {}", e))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            // Clean up copied file on error
            let _ = std::fs::remove_file(&copied);
            return Err(format!("cwebp failed: {}", stderr));
        }

        // 4. Delete copied original
        let _ = std::fs::remove_file(&copied);

        // 5. Return webp filename
        Ok(webp_name)
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))?
}
