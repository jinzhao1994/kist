// Generated from the Go HTTP contract. Run npm run generate:api.
export interface paths {
    "/api/v1/boxes": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes */
        get: operations["get-api-v1-boxes"];
        put?: never;
        /** Post API v1 boxes */
        post: operations["post-api-v1-boxes"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes by ID */
        get: operations["get-api-v1-boxes-by-id"];
        /** Put API v1 boxes by ID */
        put: operations["put-api-v1-boxes-by-id"];
        post?: never;
        /** Delete API v1 boxes by ID */
        delete: operations["delete-api-v1-boxes-by-id"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{id}/deletion-preview": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes by ID deletion preview */
        get: operations["get-api-v1-boxes-by-id-deletion-preview"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{id}/folders": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes by ID folders */
        get: operations["get-api-v1-boxes-by-id-folders"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{id}/tags": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes by ID tags */
        get: operations["get-api-v1-boxes-by-id-tags"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{id}/uploads": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Upload a complete batch with a JSON manifest followed by file parts
         * @description The first multipart field is manifest: {files:[{path,last_modified}]}, with modification times in Unix milliseconds. Remaining fields are files, with filenames matching manifest paths. Retries bind paths, timestamps and content hashes, not multipart boundaries.
         */
        post: operations["upload-files"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files */
        get: operations["get-api-v1-files"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID */
        get: operations["get-api-v1-files-by-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/content": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Read original file bytes; supports HTTP ranges */
        get: operations["GET-file-content"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        /** Read original file bytes; supports HTTP ranges */
        head: operations["HEAD-file-content"];
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/derived/{recipe}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID derived by recipe */
        get: operations["get-api-v1-files-by-id-derived-by-recipe"];
        put?: never;
        /** Post API v1 files by ID derived by recipe */
        post: operations["post-api-v1-files-by-id-derived-by-recipe"];
        /** Delete API v1 files by ID derived by recipe */
        delete: operations["delete-api-v1-files-by-id-derived-by-recipe"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/derived/{recipe}/content": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Read completed derived media; supports HTTP ranges */
        get: operations["GET-derived-content"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        /** Read completed derived media; supports HTTP ranges */
        head: operations["HEAD-derived-content"];
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/extractions/{extractor}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 files by ID extractions by extractor */
        post: operations["post-api-v1-files-by-id-extractions-by-extractor"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/history/{op}/boxes/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 history by op boxes by ID */
        get: operations["get-api-v1-history-by-op-boxes-by-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/history/{op}/files/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 history by op files by ID */
        get: operations["get-api-v1-history-by-op-files-by-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/info": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 info */
        get: operations["get-api-v1-info"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ops": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ops */
        get: operations["get-api-v1-ops"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ops/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ops by ID */
        get: operations["get-api-v1-ops-by-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ops/{id}/undo": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ops by ID undo */
        post: operations["post-api-v1-ops-by-id-undo"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/requests/{key}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 requests by key */
        get: operations["get-api-v1-requests-by-key"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 tags */
        get: operations["get-api-v1-tags"];
        put?: never;
        /** Post API v1 tags */
        post: operations["post-api-v1-tags"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /** Delete API v1 tags by ID */
        delete: operations["delete-api-v1-tags-by-id"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags/{id}/boxes": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put: operations["PUT-tag-boxes"];
        post?: never;
        delete: operations["DELETE-tag-boxes"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags/{id}/files": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put: operations["PUT-tag-files"];
        post?: never;
        delete: operations["DELETE-tag-files"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags/{id}/name": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Put API v1 tags by ID name */
        put: operations["put-api-v1-tags-by-id-name"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags/{id}/parent": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Put API v1 tags by ID parent */
        put: operations["put-api-v1-tags-by-id-parent"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        AttachTagInputBody: {
            ids: string[] | null;
        };
        Box: {
            /** Format: date-time */
            created_at: string;
            description?: string;
            end_at?: string;
            id: string;
            name: string;
            start_at?: string;
            /** Format: date-time */
            updated_at: string;
        };
        BoxBody: {
            description?: string;
            end_at?: string;
            name: string;
            start_at?: string;
        };
        BoxDeletionPreview: {
            /** Format: int64 */
            file_count: number;
            name: string;
            /** Format: int64 */
            revision: number;
        };
        BoxList: {
            boxes: components["schemas"]["Box"][] | null;
        };
        CreateTagInputBody: {
            name: string;
            parent_id?: string;
        };
        "Delete-api-v1-boxes-by-idRequest": {
            /** Format: int64 */
            revision: number;
        };
        ExtractionInputBody: Record<string, never>;
        File: {
            blob_key: string;
            box_id?: string;
            /** Format: date-time */
            captured_at: string | null;
            /** Format: date-time */
            created_at: string;
            extraction_pending?: string[] | null;
            folder_id: string;
            id: string;
            kind?: string;
            media_type?: string;
            name: string;
            path?: string;
            /** Format: date-time */
            updated_at: string;
        };
        FileDetail: {
            captured_from?: string;
            captured_zone?: string;
            extracted: string[] | null;
            file: components["schemas"]["File"];
            metadata: components["schemas"]["Metadata"][] | null;
            tags: string[] | null;
        };
        FileList: {
            cursor?: string;
            files: components["schemas"]["File"][] | null;
        };
        Folder: {
            /** Format: date-time */
            created_at: string;
            id: string;
            name: string;
            parent_id: string | null;
            /** Format: date-time */
            updated_at: string;
        };
        FolderList: {
            folders: components["schemas"]["Folder"][] | null;
        };
        HTTPError: {
            /** Format: int64 */
            blocking_op?: number;
            candidates?: components["schemas"]["Ref"][] | null;
            class: string;
            code: string;
            existing_id?: string;
            holder?: string;
            message: string;
            /** Format: int64 */
            op_id?: number;
            /** Format: int64 */
            reverted_by?: number;
        };
        HistoricalBox: {
            box: components["schemas"]["Box"];
            box_tags: string[] | null;
            cursor?: string;
            files: components["schemas"]["File"][] | null;
            folders: components["schemas"]["Folder"][] | null;
            /** Format: int64 */
            op: number;
            tags: components["schemas"]["Tag"][] | null;
        };
        HistoricalFile: {
            file: components["schemas"]["FileDetail"];
            /** Format: int64 */
            op: number;
            tags: components["schemas"]["Tag"][] | null;
        };
        HistoryDetail: {
            action: string;
            boxes: string[] | null;
            files: string[] | null;
            /** Format: int64 */
            id: number;
            origin: string;
            payload: {
                [key: string]: unknown;
            };
            /** Format: int64 */
            reverts_id: number | null;
            /** Format: int64 */
            root_op_id?: number;
            /** Format: date-time */
            ts: string;
            undoable: boolean;
            /** Format: int64 */
            undone_by: number | null;
        };
        HistoryList: {
            /** Format: int64 */
            before?: number;
            ops: components["schemas"]["HistoryOp"][] | null;
        };
        HistoryOp: {
            action: string;
            /** Format: int64 */
            id: number;
            origin: string;
            /** Format: int64 */
            reverts_id: number | null;
            /** Format: int64 */
            root_op_id?: number;
            /** Format: date-time */
            ts: string;
            undoable: boolean;
            /** Format: int64 */
            undone_by: number | null;
        };
        LibraryInfo: {
            /** Format: int64 */
            max_upload_bytes: number;
            /** Format: int64 */
            max_upload_files: number;
            projection_dirty: boolean;
            timezone: string;
            version: string;
        };
        MediaJob: {
            blob_key: string;
            generation: string;
            reason?: string;
            recipe: string;
            result: components["schemas"]["Result"];
            /** @enum {string} */
            state: "missing" | "queued" | "running" | "ready" | "blocked" | "failed" | "cancelled";
            /** Format: date-time */
            updated_at: string;
        };
        MediaRequestBody: {
            retry?: boolean;
        };
        Metadata: {
            key: string;
            source: string;
            type: string;
            value: string;
        };
        MoveTagInputBody: {
            parent_id: string | null;
        };
        Ref: {
            id: string;
            name: string;
        };
        RenameTagInputBody: {
            name: string;
        };
        RequestResult: {
            /** Format: int64 */
            op_id: number;
        };
        Result: {
            /** Format: int64 */
            bytes: number;
            engine: string;
            /** Format: int64 */
            generation_ms: number;
            /** Format: int64 */
            height: number;
            /** Format: int64 */
            width: number;
        };
        Tag: {
            /** Format: int64 */
            box_count?: number;
            children?: components["schemas"]["Tag"][] | null;
            /** Format: int64 */
            count: number | null;
            /** Format: int64 */
            file_count?: number;
            id: string;
            name: string;
            parent_id: string | null;
        };
        TagList: {
            tags: string[] | null;
        };
        TagTree: {
            tags: components["schemas"]["Tag"][] | null;
        };
        UndoInputBody: Record<string, never>;
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    "get-api-v1-boxes": {
        parameters: {
            query?: {
                tag?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BoxList"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-boxes": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["BoxBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Box"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-boxes-by-id": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["BoxBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "delete-api-v1-boxes-by-id": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Delete-api-v1-boxes-by-idRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes-by-id-deletion-preview": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BoxDeletionPreview"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes-by-id-folders": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FolderList"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes-by-id-tags": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TagList"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "upload-files": {
        parameters: {
            query?: {
                folder?: string;
            };
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "multipart/form-data": string;
            };
        };
        responses: {
            /** @description Committed request receipt */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files": {
        parameters: {
            query?: {
                kind?: string[] | null;
                box?: string[] | null;
                folder?: string;
                tag_all?: string[] | null;
                tag_any?: string[] | null;
                tag_not?: string[] | null;
                name?: string;
                from?: string;
                to?: string;
                sort?: "captured" | "imported" | "name";
                asc?: boolean;
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FileList"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FileDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "GET-file-content": {
        parameters: {
            query?: {
                download?: boolean;
                at?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Original file bytes */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/octet-stream": string;
                };
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/octet-stream": string;
                };
            };
            /** @description Unsatisfiable byte range */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "HEAD-file-content": {
        parameters: {
            query?: {
                download?: boolean;
                at?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Original file bytes */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/octet-stream": string;
                };
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/octet-stream": string;
                };
            };
            /** @description Unsatisfiable byte range */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "get-api-v1-files-by-id-derived-by-recipe": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                recipe: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["MediaJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-files-by-id-derived-by-recipe": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                recipe: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["MediaRequestBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["MediaJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "delete-api-v1-files-by-id-derived-by-recipe": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                recipe: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["MediaJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "GET-derived-content": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                recipe: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Derived image or video */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "image/jpeg": string;
                    "video/mp4": string;
                };
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Unsatisfiable byte range */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "HEAD-derived-content": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                recipe: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Derived image or video */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "image/jpeg": string;
                    "video/mp4": string;
                };
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Unsatisfiable byte range */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "post-api-v1-files-by-id-extractions-by-extractor": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
                extractor: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExtractionInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-history-by-op-boxes-by-id": {
        parameters: {
            query?: {
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path: {
                op: number;
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HistoricalBox"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-history-by-op-files-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                op: number;
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HistoricalFile"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-info": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["LibraryInfo"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ops": {
        parameters: {
            query?: {
                box?: string;
                file?: string;
                action?: string;
                group?: number;
                before?: number;
                limit?: number;
                trash?: boolean;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HistoryList"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ops-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: number;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HistoryDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ops-by-id-undo": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: number;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["UndoInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-requests-by-key": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                key: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-tags": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TagTree"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-tags": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreateTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "delete-api-v1-tags-by-id": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "PUT-tag-boxes": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AttachTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "DELETE-tag-boxes": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AttachTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "PUT-tag-files": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AttachTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "DELETE-tag-files": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AttachTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-tags-by-id-name": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["RenameTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-tags-by-id-parent": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["MoveTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
}
