import axios from 'axios';

export interface VaultFolder {
  id: string;
  user_id: string;
  parent_id: string | null;
  name: string;
  icon: string;
  color: string;
  position: number;
  is_shared: boolean;
  created_at: string;
  updated_at: string;
  entry_count?: number;
  children?: VaultFolder[];
}

export interface CreateFolderRequest {
  name: string;
  parent_id?: string;
  icon?: string;
  color?: string;
}

export interface UpdateFolderRequest {
  name?: string;
  parent_id?: string;
  icon?: string;
  color?: string;
  position?: number;
}

export interface FoldersResponse {
  folders: VaultFolder[];
}

// Folder API service
export const folderApi = {
  // Get all folders (flat or tree)
  async getFolders(tree: boolean = false): Promise<VaultFolder[]> {
    const response = await axios.get<FoldersResponse>(`/folders${tree ? '?tree=true' : ''}`);
    return response.data.folders || [];
  },

  // Get single folder
  async getFolder(id: string): Promise<VaultFolder> {
    const response = await axios.get<VaultFolder>(`/folders/${id}`);
    return response.data;
  },

  // Create folder
  async createFolder(data: CreateFolderRequest): Promise<VaultFolder> {
    const response = await axios.post<VaultFolder>('/folders', data);
    return response.data;
  },

  // Update folder
  async updateFolder(id: string, data: UpdateFolderRequest): Promise<VaultFolder> {
    const response = await axios.put<VaultFolder>(`/folders/${id}`, data);
    return response.data;
  },

  // Delete folder
  async deleteFolder(id: string, recursive: boolean = false): Promise<void> {
    await axios.delete(`/folders/${id}${recursive ? '?recursive=true' : ''}`);
  },

  // Reorder folders
  async reorderFolders(positions: Record<string, number>): Promise<void> {
    await axios.post('/folders/reorder', { positions });
  },
};

export default folderApi;
