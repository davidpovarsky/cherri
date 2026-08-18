import fs from 'node:fs';
import path from 'node:path';

const outputDirectory = path.resolve('../CherriApp/Resources/PreviewShortcut');
const indexPath = path.join(outputDirectory, 'index.html');

let html = fs.readFileSync(indexPath, 'utf8');

// WKWebView blocks ES-module scripts loaded from file:// in configurations where
// classic local scripts work. Vite's current single-file bundle is valid classic
// JavaScript too, so remove only the module/crossorigin attributes while keeping
// preview-shortcut itself completely upstream and bundled locally.
html = html
  .replace(/<script type="module" crossorigin src="([^"]+)"><\/script>/g, '<script src="$1"></script>')
  .replace(/<link rel="stylesheet" crossorigin href="([^"]+)">/g, '<link rel="stylesheet" href="$1">');

if (html.includes('type="module"')) {
  throw new Error('Preview bundle still contains an ES-module script tag.');
}

fs.writeFileSync(indexPath, html);

const javascriptFiles = fs
  .readdirSync(path.join(outputDirectory, 'assets'))
  .filter((name) => name.endsWith('.js'));

if (javascriptFiles.length !== 1) {
  throw new Error(`Expected one preview JavaScript bundle, found ${javascriptFiles.length}.`);
}

const javascript = fs.readFileSync(path.join(outputDirectory, 'assets', javascriptFiles[0]), 'utf8');
if (/\b(?:import|export)\s+(?:\{|\*|default|from|["'])/.test(javascript)) {
  throw new Error('Preview JavaScript still contains module syntax and cannot be loaded as a classic file:// script.');
}

console.log('Prepared preview-shortcut bundle for local WKWebView loading.');
