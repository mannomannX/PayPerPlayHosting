/**
 * File Upload Component for PayPerPlay
 *
 * Reusable Alpine.js component for uploading and managing server files:
 * - Resource Packs
 * - Data Packs
 * - Server Icons
 * - World Generation configs
 *
 * Usage:
 * <div x-data="fileUploader(serverId, 'resource_pack', 'Resource Pack')">
 *     <div x-html="renderUploadZone()"></div>
 *     <div x-html="renderFileList()"></div>
 * </div>
 */

/**
 * File type configuration with validation rules
 */
const FILE_TYPE_CONFIG = {
    resource_pack: {
        label: 'Resource Pack',
        icon: '📦',
        accept: '.zip',
        maxSizeMB: 100,
        description: 'ZIP file containing pack.mcmeta (max 100 MB)',
        color: 'blue'
    },
    data_pack: {
        label: 'Data Pack',
        icon: '📊',
        accept: '.zip',
        maxSizeMB: 50,
        description: 'ZIP file with pack.mcmeta and /data/ folder (max 50 MB)',
        color: 'purple'
    },
    server_icon: {
        label: 'Server Icon',
        icon: '🖼️',
        accept: '.png',
        maxSizeMB: 1,
        description: 'PNG image, exactly 64x64 pixels (max 1 MB)',
        color: 'green'
    },
    world_gen: {
        label: 'World Generation',
        icon: '🌍',
        accept: '.json',
        maxSizeMB: 5,
        description: 'JSON configuration file (max 5 MB)',
        color: 'yellow'
    }
};

/**
 * Create a file uploader component
 * @param {string} serverId - Server ID
 * @param {string} fileType - File type (resource_pack, data_pack, server_icon, world_gen)
 * @param {string} title - Display title (optional)
 */
function fileUploader(serverId, fileType, title = null) {
    const config = FILE_TYPE_CONFIG[fileType];
    if (!config) {
        console.error('Unknown file type:', fileType);
        return {};
    }

    return {
        serverId: serverId,
        fileType: fileType,
        title: title || config.label,
        config: config,

        // State
        files: [],
        selectedFile: null,
        uploading: false,
        uploadProgress: 0,
        error: null,
        success: null,
        dragOver: false,
        autoActivate: true,

        // Initialize
        async init() {
            await this.loadFiles();
        },

        /**
         * Load files from server
         */
        async loadFiles() {
            try {
                const token = localStorage.getItem('token');
                const response = await fetch(`/api/servers/${this.serverId}/uploads?type=${this.fileType}`, {
                    headers: {
                        'Authorization': `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    this.files = await response.json() || [];
                } else {
                    console.error('Failed to load files:', response.statusText);
                    this.files = [];
                }
            } catch (err) {
                console.error('Error loading files:', err);
                this.files = [];
            }
        },

        /**
         * Handle file selection
         */
        handleFileSelect(event) {
            const file = event.target.files[0];
            if (file) {
                this.validateAndUpload(file);
            }
        },

        /**
         * Handle drag & drop
         */
        handleDrop(event) {
            event.preventDefault();
            this.dragOver = false;

            const file = event.dataTransfer.files[0];
            if (file) {
                this.validateAndUpload(file);
            }
        },

        handleDragOver(event) {
            event.preventDefault();
            this.dragOver = true;
        },

        handleDragLeave() {
            this.dragOver = false;
        },

        /**
         * Validate file and upload
         */
        async validateAndUpload(file) {
            this.error = null;
            this.success = null;

            // Validate file extension
            const extension = '.' + file.name.split('.').pop().toLowerCase();
            if (!this.config.accept.includes(extension)) {
                this.error = `Invalid file type. Expected: ${this.config.accept}`;
                return;
            }

            // Validate file size
            const sizeMB = file.size / 1024 / 1024;
            if (sizeMB > this.config.maxSizeMB) {
                this.error = `File too large. Maximum size: ${this.config.maxSizeMB} MB (selected: ${sizeMB.toFixed(2)} MB)`;
                return;
            }

            // Additional validation for server icons (64x64 PNG)
            if (this.fileType === 'server_icon') {
                try {
                    await this.validateImageSize(file);
                } catch (err) {
                    this.error = err.message;
                    return;
                }
            }

            // Upload file
            await this.uploadFile(file);
        },

        /**
         * Validate image dimensions (for server icons)
         */
        validateImageSize(file) {
            return new Promise((resolve, reject) => {
                const reader = new FileReader();
                reader.onload = (e) => {
                    const img = new Image();
                    img.onload = () => {
                        if (img.width === 64 && img.height === 64) {
                            resolve();
                        } else {
                            reject(new Error(`Server icon must be exactly 64x64 pixels. Selected image is ${img.width}x${img.height}.`));
                        }
                    };
                    img.onerror = () => reject(new Error('Failed to load image'));
                    img.src = e.target.result;
                };
                reader.readAsDataURL(file);
            });
        },

        /**
         * Upload file to server
         */
        async uploadFile(file) {
            this.uploading = true;
            this.uploadProgress = 0;
            this.error = null;

            const formData = new FormData();
            formData.append('file', file);
            formData.append('type', this.fileType);
            formData.append('auto_activate', this.autoActivate ? 'true' : 'false');

            try {
                const token = localStorage.getItem('token');
                const xhr = new XMLHttpRequest();

                // Track upload progress
                xhr.upload.addEventListener('progress', (e) => {
                    if (e.lengthComputable) {
                        this.uploadProgress = Math.round((e.loaded / e.total) * 100);
                    }
                });

                // Handle completion
                xhr.addEventListener('load', async () => {
                    this.uploading = false;
                    if (xhr.status === 201) {
                        this.success = `${this.config.label} uploaded successfully!`;
                        await this.loadFiles();

                        // Clear success message after 3 seconds
                        setTimeout(() => {
                            this.success = null;
                        }, 3000);
                    } else {
                        const error = JSON.parse(xhr.responseText);
                        this.error = error.error || 'Upload failed';
                    }
                });

                // Handle errors
                xhr.addEventListener('error', () => {
                    this.uploading = false;
                    this.error = 'Upload failed. Please try again.';
                });

                // Send request
                xhr.open('POST', `/api/servers/${this.serverId}/uploads`);
                xhr.setRequestHeader('Authorization', `Bearer ${token}`);
                xhr.send(formData);

            } catch (err) {
                this.uploading = false;
                this.error = 'Upload failed: ' + err.message;
            }
        },

        /**
         * Activate a file
         */
        async activateFile(fileId) {
            try {
                const token = localStorage.getItem('token');
                const response = await fetch(`/api/servers/${this.serverId}/uploads/${fileId}/activate`, {
                    method: 'PUT',
                    headers: {
                        'Authorization': `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    await this.loadFiles();
                    this.success = 'File activated successfully!';
                    setTimeout(() => this.success = null, 3000);
                } else {
                    const error = await response.json();
                    this.error = error.error || 'Failed to activate file';
                }
            } catch (err) {
                this.error = 'Failed to activate file: ' + err.message;
            }
        },

        /**
         * Deactivate a file
         */
        async deactivateFile(fileId) {
            try {
                const token = localStorage.getItem('token');
                const response = await fetch(`/api/servers/${this.serverId}/uploads/${fileId}/deactivate`, {
                    method: 'PUT',
                    headers: {
                        'Authorization': `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    await this.loadFiles();
                    this.success = 'File deactivated successfully!';
                    setTimeout(() => this.success = null, 3000);
                } else {
                    const error = await response.json();
                    this.error = error.error || 'Failed to deactivate file';
                }
            } catch (err) {
                this.error = 'Failed to deactivate file: ' + err.message;
            }
        },

        /**
         * Delete a file
         */
        async deleteFile(fileId, fileName) {
            if (!confirm(`Are you sure you want to delete "${fileName}"? This action cannot be undone.`)) {
                return;
            }

            try {
                const token = localStorage.getItem('token');
                const response = await fetch(`/api/servers/${this.serverId}/uploads/${fileId}`, {
                    method: 'DELETE',
                    headers: {
                        'Authorization': `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    await this.loadFiles();
                    this.success = 'File deleted successfully!';
                    setTimeout(() => this.success = null, 3000);
                } else {
                    const error = await response.json();
                    this.error = error.error || 'Failed to delete file';
                }
            } catch (err) {
                this.error = 'Failed to delete file: ' + err.message;
            }
        },

        /**
         * Download a file
         */
        downloadFile(fileId, fileName) {
            const token = localStorage.getItem('token');
            const url = `/api/servers/${this.serverId}/uploads/${fileId}`;

            // Create hidden link and trigger download
            const link = document.createElement('a');
            link.href = url;
            link.download = fileName;
            link.style.display = 'none';
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
        },

        /**
         * Format file size
         */
        formatSize(sizeMB) {
            if (sizeMB < 1) {
                return `${(sizeMB * 1024).toFixed(1)} KB`;
            }
            return `${sizeMB.toFixed(2)} MB`;
        },

        /**
         * Format date
         */
        formatDate(dateString) {
            const date = new Date(dateString);
            return date.toLocaleString('de-DE', {
                year: 'numeric',
                month: '2-digit',
                day: '2-digit',
                hour: '2-digit',
                minute: '2-digit'
            });
        },

        /**
         * Get status badge color
         */
        getStatusColor(status) {
            const colors = {
                'active': 'bg-green-500',
                'inactive': 'bg-gray-500',
                'uploading': 'bg-yellow-500',
                'processing': 'bg-blue-500',
                'failed': 'bg-red-500'
            };
            return colors[status] || 'bg-gray-500';
        },

        /**
         * Get color classes for this file type
         */
        getColorClasses() {
            const colors = {
                'blue': {
                    border: 'border-blue-500',
                    bg: 'bg-blue-500',
                    text: 'text-blue-400',
                    hover: 'hover:bg-blue-600'
                },
                'purple': {
                    border: 'border-purple-500',
                    bg: 'bg-purple-500',
                    text: 'text-purple-400',
                    hover: 'hover:bg-purple-600'
                },
                'green': {
                    border: 'border-green-500',
                    bg: 'bg-green-500',
                    text: 'text-green-400',
                    hover: 'hover:bg-green-600'
                },
                'yellow': {
                    border: 'border-yellow-500',
                    bg: 'bg-yellow-500',
                    text: 'text-yellow-400',
                    hover: 'hover:bg-yellow-600'
                }
            };
            return colors[this.config.color] || colors.blue;
        }
    };
}

// Export for use in HTML
if (typeof window !== 'undefined') {
    window.fileUploader = fileUploader;
    window.FILE_TYPE_CONFIG = FILE_TYPE_CONFIG;
}
