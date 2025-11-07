# File Upload Component

Wiederverwendbare Alpine.js Komponente für das Hochladen und Verwalten von Minecraft Server-Dateien.

## Features

✅ **4 Dateitypen unterstützt:**
- 📦 Resource Packs (ZIP, max 100 MB)
- 📊 Data Packs (ZIP, max 50 MB)
- 🖼️ Server Icons (PNG 64x64, max 1 MB)
- 🌍 World Generation Configs (JSON, max 5 MB)

✅ **Funktionen:**
- Drag & Drop Upload
- Upload-Fortschrittsanzeige
- Client-seitige Validierung (Dateityp, Größe, Dimensionen)
- Datei-Aktivierung/Deaktivierung (nur eine aktiv pro Typ)
- Versionierung und Metadaten
- SHA1 Hash-Verifizierung
- Download und Löschen
- Auto-Aktivierungs-Option
- Responsive Design mit TailwindCSS

## Installation

### 1. Scripts einbinden

```html
<script src="/static/js/file-upload-component.js"></script>
<script src="/static/js/file-upload-ui.js"></script>
```

### 2. Alpine.js Setup

Die Komponente benötigt Alpine.js 3.x:

```html
<script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
```

### 3. Render-Methoden verfügbar machen

```html
<script>
    // Fügt die UI-Render-Methoden zur fileUploader-Funktion hinzu
    Object.assign(window.fileUploader, window.fileUploaderUI);
</script>
```

## Verwendung

### Einfache Verwendung

```html
<div x-data="fileUploader('server-id', 'resource_pack')" x-init="init()">
    <div x-html="renderUploadZone.call($data)"></div>
    <div x-html="renderFileList.call($data)"></div>
</div>
```

### Mit Custom Titel

```html
<div x-data="fileUploader('server-id', 'data_pack', 'Mein Custom Data Pack')"
     x-init="init()">
    <div x-html="renderUploadZone.call($data)"></div>
    <div x-html="renderFileList.call($data)"></div>
</div>
```

### Kompakte Variante (für Modals/kleine Bereiche)

```html
<div x-data="fileUploader('server-id', 'server_icon')" x-init="init()">
    <div x-html="renderCompactUploader.call($data)"></div>
</div>
```

### Tabbed Interface

```html
<div x-data="{ serverId: 'my-server', activeTab: 'resource_pack' }">
    <!-- Tabs -->
    <div class="flex gap-2 mb-6 border-b border-gray-700">
        <template x-for="(config, type) in FILE_TYPE_CONFIG" :key="type">
            <button @click="activeTab = type"
                    :class="activeTab === type ? 'border-green-400' : 'border-transparent'"
                    class="px-4 py-2 border-b-2">
                <span x-text="config.icon"></span>
                <span x-text="config.label"></span>
            </button>
        </template>
    </div>

    <!-- Tab Contents -->
    <div x-show="activeTab === 'resource_pack'"
         x-data="fileUploader(serverId, 'resource_pack')"
         x-init="init()">
        <div x-html="renderUploadZone.call($data)"></div>
        <div x-html="renderFileList.call($data)"></div>
    </div>

    <div x-show="activeTab === 'data_pack'"
         x-data="fileUploader(serverId, 'data_pack')"
         x-init="init()">
        <div x-html="renderUploadZone.call($data)"></div>
        <div x-html="renderFileList.call($data)"></div>
    </div>

    <!-- ... weitere Tabs -->
</div>
```

## API

### Komponenten-Parameter

```javascript
fileUploader(serverId, fileType, title)
```

- **serverId** (string, required): Die Server-ID
- **fileType** (string, required): Der Dateityp
  - `'resource_pack'` - Resource Packs
  - `'data_pack'` - Data Packs
  - `'server_icon'` - Server Icons
  - `'world_gen'` - World Generation Configs
- **title** (string, optional): Custom Titel für die Anzeige

### Render-Methoden

#### renderUploadZone()
Rendert die Upload-Zone mit Drag & Drop und Fortschrittsanzeige.

```html
<div x-html="renderUploadZone.call($data)"></div>
```

#### renderFileList()
Rendert die Liste der hochgeladenen Dateien mit Aktionen.

```html
<div x-html="renderFileList.call($data)"></div>
```

#### renderCompactUploader()
Rendert eine kompakte Upload-Variante ohne Drag & Drop Zone.

```html
<div x-html="renderCompactUploader.call($data)"></div>
```

### Komponenten-Methoden

Diese Methoden sind intern verfügbar und werden automatisch aufgerufen:

- `init()` - Initialisiert die Komponente und lädt Dateien
- `loadFiles()` - Lädt die Liste der Dateien vom Server
- `handleFileSelect(event)` - Verarbeitet Datei-Auswahl
- `handleDrop(event)` - Verarbeitet Drag & Drop
- `validateAndUpload(file)` - Validiert und lädt Datei hoch
- `activateFile(fileId)` - Aktiviert eine Datei
- `deactivateFile(fileId)` - Deaktiviert eine Datei
- `deleteFile(fileId, fileName)` - Löscht eine Datei
- `downloadFile(fileId, fileName)` - Lädt eine Datei herunter

## Backend API Endpoints

Die Komponente nutzt folgende API-Endpunkte:

```
POST   /api/servers/:id/uploads
       - Upload neue Datei
       - Multipart form: file, type, auto_activate

GET    /api/servers/:id/uploads?type=resource_pack
       - Liste alle Dateien (optional gefiltert nach Typ)

GET    /api/servers/:id/uploads/:fileId
       - Download eine spezifische Datei

PUT    /api/servers/:id/uploads/:fileId/activate
       - Aktiviere eine Datei (deaktiviert andere des gleichen Typs)

PUT    /api/servers/:id/uploads/:fileId/deactivate
       - Deaktiviere eine Datei

DELETE /api/servers/:id/uploads/:fileId
       - Lösche eine Datei permanent
```

## Validierungsregeln

### Resource Packs
- Dateityp: `.zip`
- Max. Größe: 100 MB
- Muss `pack.mcmeta` enthalten

### Data Packs
- Dateityp: `.zip`
- Max. Größe: 50 MB
- Muss `pack.mcmeta` und `/data/` Ordner enthalten

### Server Icons
- Dateityp: `.png`
- Max. Größe: 1 MB
- Exakte Dimensionen: 64x64 Pixel

### World Generation
- Dateityp: `.json`
- Max. Größe: 5 MB
- Valide JSON-Struktur

## Styling

Die Komponente nutzt TailwindCSS. Jeder Dateityp hat eigene Farbschemata:

- **Resource Packs**: Blau (`blue-500`)
- **Data Packs**: Lila (`purple-500`)
- **Server Icons**: Grün (`green-500`)
- **World Gen**: Gelb (`yellow-500`)

## Beispiele

Siehe [`file-upload-example.html`](../file-upload-example.html) für vollständige, funktionierende Beispiele:

1. Tabbed Interface mit allen 4 Dateitypen
2. Standalone Resource Pack Uploader
3. Kompakte Uploader für Modals

## Troubleshooting

### Dateien werden nicht geladen

Stelle sicher, dass:
1. Der Benutzer authentifiziert ist (JWT Token in localStorage)
2. Die Server-ID korrekt ist
3. Die API-Endpunkte erreichbar sind

### Upload schlägt fehl

Prüfe:
1. Dateigröße (siehe Limits oben)
2. Dateityp (muss exakt dem `accept`-Attribut entsprechen)
3. Für Icons: Dimensionen müssen exakt 64x64 sein
4. Server-Logs für Backend-Fehler

### "x-html" funktioniert nicht

Stelle sicher, dass die Render-Methoden mit `.call($data)` aufgerufen werden:

```html
<!-- ✅ Richtig -->
<div x-html="renderUploadZone.call($data)"></div>

<!-- ❌ Falsch -->
<div x-html="renderUploadZone()"></div>
```

## Integration in index.html

Um die Komponente in die Haupt-Anwendung zu integrieren:

1. Scripts in `<head>` einbinden
2. Render-Methoden Setup im Footer
3. Komponente im Server-Detail-Bereich verwenden

Beispiel siehe `file-upload-example.html`.

## Lizenz

Teil des PayPerPlay Hosting Systems.
