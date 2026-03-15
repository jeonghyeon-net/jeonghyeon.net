import ReactDOM from "react-dom/client";
import App from "./App";

// Prevent WKWebView overscroll bounce on document-level scroll
document.addEventListener("wheel", (e) => {
  const target = e.target as HTMLElement;
  // Allow scrolling inside scrollable elements, block on document body
  let el: HTMLElement | null = target;
  while (el && el !== document.body) {
    const { overflowY } = getComputedStyle(el);
    if ((overflowY === "auto" || overflowY === "scroll") && el.scrollHeight > el.clientHeight) {
      return; // scrollable container, allow
    }
    el = el.parentElement;
  }
  e.preventDefault();
}, { passive: false });

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <App />,
);
