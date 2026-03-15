import { useState, useEffect, useRef } from "react";
import { invoke } from "@tauri-apps/api/core";
import { convertFileSrc } from "@tauri-apps/api/core";

interface PreviewProps {
  projectPath: string;
  filePath: string | null;
  triggerRender: number;
}

function Preview({ projectPath, filePath, triggerRender }: PreviewProps) {
  const [html, setHtml] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const lastHtmlRef = useRef<string>("");

  useEffect(() => {
    if (!filePath) return;

    const timer = setTimeout(async () => {
      try {
        const rendered = await invoke<string>("render_single_file", {
          projectPath,
          mdPath: filePath,
        });

        // Replace image paths with Tauri asset:// protocol URLs
        const fileDir = filePath.substring(0, filePath.lastIndexOf("/"));
        const contentDir = `${projectPath}/content`;
        const processedHtml = rendered.replace(
          /src="([^"]+\.(webp|png|jpg|jpeg|gif))"/gi,
          (match, src) => {
            if (src.startsWith("http") || src.startsWith("asset:")) {
              return match;
            }
            // Absolute paths (/) resolve from content dir
            const absPath = src.startsWith("/")
              ? `${contentDir}${src}`
              : `${fileDir}/${src}`;
            return `src="${convertFileSrc(absPath)}"`;
          }
        );

        // Inject styles and force light color scheme in preview
        const withInjections = processedHtml
          .replace(
            '<meta name="color-scheme" content="light dark">',
            '<meta name="color-scheme" content="light">'
          )
          .replace(
            "</head>",
            "<style>html,body{overscroll-behavior:none;color-scheme:light}</style></head>"
          );
        setHtml(withInjections);
        lastHtmlRef.current = processedHtml;
        setError(null);
      } catch (e) {
        setError(String(e));
        // Keep last successful HTML
        if (lastHtmlRef.current) {
          setHtml(lastHtmlRef.current);
        }
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [projectPath, filePath, triggerRender]);

  if (!filePath) {
    return <div className="preview-empty">Preview</div>;
  }

  return (
    <div className="preview-container">
      {error && <div className="preview-error">{error}</div>}
      <iframe
        className="preview-iframe"
        srcDoc={html}
        title="Preview"
        sandbox="allow-same-origin"
      />
    </div>
  );
}

export default Preview;
