/**
 * File Upload UI Templates
 *
 * Provides HTML template functions for the file upload component
 * These can be used with Alpine.js x-html directive
 */

/**
 * Render the upload zone with drag & drop
 */
function renderUploadZone() {
    const colors = this.getColorClasses();

    return `
        <div class="bg-gray-800 rounded-lg p-6 mb-6">
            <div class="flex items-start justify-between mb-4">
                <div>
                    <h3 class="text-xl font-bold ${colors.text} flex items-center gap-2">
                        <span>${this.config.icon}</span>
                        <span>${this.title}</span>
                    </h3>
                    <p class="text-gray-400 text-sm mt-1">${this.config.description}</p>
                </div>
                <label class="flex items-center gap-2 text-sm text-gray-400 cursor-pointer">
                    <input type="checkbox" x-model="autoActivate" class="rounded">
                    <span>Auto-activate</span>
                </label>
            </div>

            <!-- Upload Zone -->
            <div class="relative">
                <input type="file"
                       accept="${this.config.accept}"
                       @change="handleFileSelect($event)"
                       class="hidden"
                       id="file-input-${this.fileType}">

                <label for="file-input-${this.fileType}"
                       @drop="handleDrop($event)"
                       @dragover="handleDragOver($event)"
                       @dragleave="handleDragLeave()"
                       :class="dragOver ? 'border-${this.config.color}-500 bg-${this.config.color}-500 bg-opacity-10' : 'border-gray-600'"
                       class="flex flex-col items-center justify-center border-2 border-dashed rounded-lg p-8 cursor-pointer transition hover:border-${this.config.color}-500 hover:bg-${this.config.color}-500 hover:bg-opacity-5">

                    <svg class="w-12 h-12 text-gray-500 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"></path>
                    </svg>

                    <p class="text-gray-300 text-center mb-1">
                        <span class="${colors.text} font-semibold">Click to upload</span> or drag and drop
                    </p>
                    <p class="text-gray-500 text-sm">${this.config.accept.toUpperCase()} • Max ${this.config.maxSizeMB} MB</p>
                </label>
            </div>

            <!-- Upload Progress -->
            <div x-show="uploading" class="mt-4">
                <div class="flex items-center justify-between mb-2">
                    <span class="text-sm text-gray-400">Uploading...</span>
                    <span class="text-sm ${colors.text} font-semibold" x-text="uploadProgress + '%'"></span>
                </div>
                <div class="w-full bg-gray-700 rounded-full h-2 overflow-hidden">
                    <div class="${colors.bg} h-full transition-all duration-300"
                         :style="'width: ' + uploadProgress + '%'"></div>
                </div>
            </div>

            <!-- Success Message -->
            <div x-show="success"
                 class="mt-4 p-3 bg-green-500 bg-opacity-20 border border-green-500 rounded flex items-start gap-2">
                <svg class="w-5 h-5 text-green-400 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                </svg>
                <span class="text-sm text-green-400" x-text="success"></span>
            </div>

            <!-- Error Message -->
            <div x-show="error"
                 class="mt-4 p-3 bg-red-500 bg-opacity-20 border border-red-500 rounded flex items-start gap-2">
                <svg class="w-5 h-5 text-red-400 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path>
                </svg>
                <span class="text-sm text-red-400" x-text="error"></span>
            </div>
        </div>
    `;
}

/**
 * Render the list of uploaded files
 */
function renderFileList() {
    if (this.files.length === 0) {
        return `
            <div class="bg-gray-800 rounded-lg p-8 text-center">
                <div class="text-gray-500 text-4xl mb-2">${this.config.icon}</div>
                <p class="text-gray-400">No ${this.title.toLowerCase()}s uploaded yet</p>
                <p class="text-gray-500 text-sm mt-1">Upload your first file above</p>
            </div>
        `;
    }

    const colors = this.getColorClasses();
    const filesHtml = this.files.map(file => `
        <div class="bg-gray-800 rounded-lg p-4 border-l-4 ${colors.border}">
            <div class="flex items-start justify-between">
                <div class="flex-1 min-w-0 mr-4">
                    <!-- File Name & Status -->
                    <div class="flex items-center gap-2 mb-2">
                        <h4 class="text-white font-semibold truncate">${file.file_name}</h4>
                        <span class="px-2 py-0.5 text-xs rounded-full ${this.getStatusColor(file.status)} text-white">
                            ${file.status}
                        </span>
                        ${file.is_active ? '<span class="px-2 py-0.5 text-xs rounded-full bg-green-500 text-white">ACTIVE</span>' : ''}
                    </div>

                    <!-- File Info -->
                    <div class="flex flex-wrap gap-4 text-xs text-gray-400 mb-2">
                        <span class="flex items-center gap-1">
                            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"></path>
                            </svg>
                            ${this.formatSize(file.size_mb)}
                        </span>
                        <span class="flex items-center gap-1">
                            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                            </svg>
                            ${this.formatDate(file.uploaded_at)}
                        </span>
                        <span class="flex items-center gap-1">
                            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"></path>
                            </svg>
                            v${file.version}
                        </span>
                    </div>

                    <!-- SHA1 Hash -->
                    ${file.sha1_hash ? `
                        <div class="text-xs text-gray-500 font-mono truncate">
                            SHA1: ${file.sha1_hash}
                        </div>
                    ` : ''}

                    <!-- Error Message -->
                    ${file.error_message ? `
                        <div class="mt-2 text-xs text-red-400 bg-red-500 bg-opacity-10 border border-red-500 rounded p-2">
                            ${file.error_message}
                        </div>
                    ` : ''}
                </div>

                <!-- Actions -->
                <div class="flex flex-col gap-2">
                    ${!file.is_active && file.status !== 'failed' ? `
                        <button @click="activateFile('${file.id}')"
                                class="px-3 py-1 text-xs bg-green-500 hover:bg-green-600 text-white rounded transition whitespace-nowrap">
                            Activate
                        </button>
                    ` : ''}

                    ${file.is_active ? `
                        <button @click="deactivateFile('${file.id}')"
                                class="px-3 py-1 text-xs bg-gray-600 hover:bg-gray-700 text-white rounded transition whitespace-nowrap">
                            Deactivate
                        </button>
                    ` : ''}

                    <button @click="downloadFile('${file.id}', '${file.file_name}')"
                            class="px-3 py-1 text-xs ${colors.bg} ${colors.hover} text-white rounded transition whitespace-nowrap">
                        Download
                    </button>

                    <button @click="deleteFile('${file.id}', '${file.file_name}')"
                            class="px-3 py-1 text-xs bg-red-500 hover:bg-red-600 text-white rounded transition whitespace-nowrap">
                        Delete
                    </button>
                </div>
            </div>
        </div>
    `).join('');

    return `
        <div>
            <div class="flex items-center justify-between mb-4">
                <h3 class="text-lg font-semibold text-white">
                    Uploaded Files (${this.files.length})
                </h3>
                <button @click="loadFiles()"
                        class="text-sm text-gray-400 hover:text-white transition flex items-center gap-1">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                    </svg>
                    Refresh
                </button>
            </div>
            <div class="space-y-3">
                ${filesHtml}
            </div>
        </div>
    `;
}

/**
 * Render a compact file selector (for tabs/modals)
 */
function renderCompactUploader() {
    const colors = this.getColorClasses();

    return `
        <div class="space-y-4">
            <!-- Compact Upload Button -->
            <div>
                <input type="file"
                       accept="${this.config.accept}"
                       @change="handleFileSelect($event)"
                       class="hidden"
                       id="compact-file-input-${this.fileType}">

                <label for="compact-file-input-${this.fileType}"
                       class="inline-flex items-center gap-2 px-4 py-2 ${colors.bg} ${colors.hover} text-white rounded cursor-pointer transition">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"></path>
                    </svg>
                    Upload ${this.title}
                </label>
                <p class="text-xs text-gray-500 mt-2">${this.config.description}</p>
            </div>

            <!-- Upload Progress -->
            <div x-show="uploading" class="space-y-2">
                <div class="flex items-center justify-between">
                    <span class="text-sm text-gray-400">Uploading...</span>
                    <span class="text-sm ${colors.text} font-semibold" x-text="uploadProgress + '%'"></span>
                </div>
                <div class="w-full bg-gray-700 rounded-full h-1.5 overflow-hidden">
                    <div class="${colors.bg} h-full transition-all duration-300"
                         :style="'width: ' + uploadProgress + '%'"></div>
                </div>
            </div>

            <!-- Messages -->
            <div x-show="success" class="text-sm text-green-400" x-text="success"></div>
            <div x-show="error" class="text-sm text-red-400" x-text="error"></div>
        </div>
    `;
}

// Attach to component prototypes
if (typeof window !== 'undefined') {
    // Make these available as methods that can be called on the component
    window.fileUploaderUI = {
        renderUploadZone,
        renderFileList,
        renderCompactUploader
    };
}
