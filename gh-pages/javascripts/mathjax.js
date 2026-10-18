window.MathJax = {
  tex: {
    inlineMath: [[String.raw`\(`, String.raw`\)`], ["$", "$"]],
    displayMath: [[String.raw`\[`, String.raw`\]`], ["$$", "$$"]],
    processEscapes: true,
    processEnvironments: true
  },
  options: {
    ignoreHtmlClass: ".*",
    processHtmlClass: "arithmatex"
  }
};
document.addEventListener("DOMContentLoaded", function () {
  MathJax.typesetPromise();
});
