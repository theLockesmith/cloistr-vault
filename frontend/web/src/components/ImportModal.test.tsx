/**
 * The import modal stays mounted while closed, so its state outlived a close:
 * after one import, reopening it showed "Import complete" again with no way
 * to pick a format or file (found by the WV4 live sweep, VAULT-17).
 */
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import ImportModal from './ImportModal';

const addEntry = vi.fn(async () => {});
vi.mock('../contexts/VaultContext', () => ({ useVault: () => ({ addEntry }) }));
vi.mock('react-hot-toast', () => {
  const toast: any = vi.fn();
  toast.success = vi.fn();
  toast.error = vi.fn();
  return { __esModule: true, default: toast };
});

describe('ImportModal', () => {
  it('starts fresh each time it is opened', async () => {
    const { rerender, container } = render(<ImportModal isOpen onClose={() => {}} />);

    fireEvent.click(screen.getByRole('button', { name: 'Generic CSV' }));
    const csv = 'name,username,password,url,notes\n"one","u","p","https://x.example","n"\n';
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [new File([csv], 'one.csv', { type: 'text/csv' })] } });
    fireEvent.click(await screen.findByRole('button', { name: /^Import 1 item$/ }));
    await screen.findByText('Import complete');
    expect(addEntry).toHaveBeenCalledTimes(1);

    rerender(<ImportModal isOpen={false} onClose={() => {}} />);
    rerender(<ImportModal isOpen onClose={() => {}} />);

    await waitFor(() => expect(screen.queryByText('Import complete')).toBeNull());
    expect(screen.getByRole('button', { name: 'KeePass CSV' })).toBeTruthy();
    expect(container.querySelector('input[type="file"]')).toBeTruthy();
  });
});
