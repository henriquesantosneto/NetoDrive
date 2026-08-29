import { FormEvent, useEffect, useMemo, useState, useTransition } from "react";
import {
  bulkDelete,
  bulkDownload,
  bulkPurge,
  bulkRestore,
  clearToken,
  createDir,
  deleteFile,
  downloadUrl,
  emptyTrash,
  FileMeta,
  formatSize,
  GalleryAlbum,
  getToken,
  listFiles,
  listGalleryAlbum,
  listGalleryAlbums,
  listTrash,
  login,
  openUrl,
  purgeFromTrash,
  restoreFromTrash,
  setToken,
  uploadFile,
} from "./api";

type Preview = { path: string; name: string; mime: string };
type View = "files" | "gallery" | "trash";

export default function App() {
  const [token, setTok] = useState(getToken());
  if (!token) {
    return (
      <Login
        onSuccess={(t) => {
          setToken(t);
          setTok(t);
        }}
      />
    );
  }
  return (
    <Drive
      onLogout={() => {
        clearToken();
        setTok("");
      }}
    />
  );
}

function Login({ onSuccess }: { onSuccess: (token: string) => void }) {
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("admin123");
  const [error, setError] = useState("");
  const [pending, start] = useTransition();

  function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    start(async () => {
      try {
        const res = await login(username, password);
        onSuccess(res.token);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Falha no login");
      }
    });
  }

  return (
    <div className="login-panel">
      <div className="login-card">
        <div className="login-brand">
          <div className="cloud">N</div>
          <div>
            <h1>NetoDrive</h1>
          </div>
        </div>
        <p className="sub">Entre para acessar seus arquivos — como no OneDrive, no seu servidor.</p>
        <form onSubmit={submit}>
          <label>
            Conta
            <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
          </label>
          <label>
            Senha
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
            />
          </label>
          <button className="btn" disabled={pending}>
            {pending ? "Entrando…" : "Entrar"}
          </button>
          {error ? <p className="error">{error}</p> : null}
        </form>
      </div>
    </div>
  );
}

function Drive({ onLogout }: { onLogout: () => void }) {
  const [view, setView] = useState<View>("files");
  const [path, setPath] = useState("");
  const [files, setFiles] = useState<FileMeta[]>([]);
  const [query, setQuery] = useState("");
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<Preview | null>(null);
  const [pending, start] = useTransition();
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const [galleryAlbums, setGalleryAlbums] = useState<GalleryAlbum[]>([]);
  const [galleryAlbumPath, setGalleryAlbumPath] = useState<string | null>(null);
  const [galleryItems, setGalleryItems] = useState<FileMeta[]>([]);

  function clearSelection() {
    setSelected(new Set());
  }

  function toggleSelect(p: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
  }

  function toggleSelectAll(items: FileMeta[]) {
    setSelected((prev) => {
      const allSelected = items.length > 0 && items.every((f) => prev.has(f.path));
      if (allSelected) return new Set();
      return new Set(items.map((f) => f.path));
    });
  }

  function loadGalleryAlbums() {
    start(async () => {
      try {
        setError("");
        clearSelection();
        setGalleryAlbumPath(null);
        setGalleryItems([]);
        const res = await listGalleryAlbums();
        setGalleryAlbums(res.albums);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Erro ao listar álbuns");
      }
    });
  }

  function openGalleryAlbum(albumPath: string) {
    start(async () => {
      try {
        setError("");
        clearSelection();
        const res = await listGalleryAlbum(albumPath);
        setGalleryAlbumPath(res.path);
        setGalleryItems(res.items);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Erro ao abrir álbum");
      }
    });
  }

  function refresh(nextPath = path) {
    start(async () => {
      try {
        setError("");
        clearSelection();
        if (view === "gallery") {
          if (galleryAlbumPath) {
            const res = await listGalleryAlbum(galleryAlbumPath);
            setGalleryAlbumPath(res.path);
            setGalleryItems(res.items);
          } else {
            const res = await listGalleryAlbums();
            setGalleryAlbums(res.albums);
          }
        } else if (view === "trash") {
          const res = await listTrash();
          setFiles(res.items);
        } else {
          const res = await listFiles(nextPath);
          setFiles(res.files);
          setPath(nextPath);
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "Erro ao listar");
      }
    });
  }

  useEffect(() => {
    if (view === "gallery") {
      loadGalleryAlbums();
    } else {
      refresh(view === "files" ? path : "");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return files;
    return files.filter((f) => f.name.toLowerCase().includes(q) || f.path.toLowerCase().includes(q));
  }, [files, query]);

  const filteredAlbums = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return galleryAlbums;
    return galleryAlbums.filter(
      (a) => a.name.toLowerCase().includes(q) || a.path.toLowerCase().includes(q),
    );
  }, [galleryAlbums, query]);

  const filteredGalleryItems = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return galleryItems;
    return galleryItems.filter(
      (f) => f.name.toLowerCase().includes(q) || f.path.toLowerCase().includes(q),
    );
  }, [galleryItems, query]);

  async function onUpload(list: FileList | null) {
    if (!list?.length) return;
    try {
      for (const file of Array.from(list)) {
        const remote = path ? `${path}/${file.name}` : file.name;
        await uploadFile(remote, file);
      }
      refresh(path);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Upload falhou");
    }
  }

  async function onMkdir() {
    const name = window.prompt("Nome da pasta");
    if (!name) return;
    const remote = path ? `${path}/${name}` : name;
    try {
      await createDir(remote);
      refresh(path);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao criar pasta");
    }
  }

  async function onDelete(f: FileMeta) {
    if (!window.confirm(`Mover "${f.name}" para a lixeira?`)) return;
    try {
      await deleteFile(f.path);
      if (preview?.path === f.path) setPreview(null);
      refresh(path);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao apagar");
    }
  }

  async function onRestore(f: FileMeta) {
    try {
      await restoreFromTrash(f.path);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao restaurar");
    }
  }

  async function onPurge(f: FileMeta) {
    if (!window.confirm(`Excluir definitivamente "${f.name}"? Isso nao pode ser desfeito.`)) return;
    try {
      await purgeFromTrash(f.path);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao excluir");
    }
  }

  async function onEmptyTrash() {
    if (!window.confirm("Esvaziar a lixeira? Todos os itens serao excluidos definitivamente.")) return;
    try {
      await emptyTrash();
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao esvaziar");
    }
  }

  async function onBulkDelete() {
    const paths = [...selected];
    if (!paths.length) return;
    if (!window.confirm(`Mover ${paths.length} item(ns) para a lixeira?`)) return;
    try {
      await bulkDelete(paths);
      refresh(path);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha na exclusao em massa");
    }
  }

  async function onBulkDownload() {
    const paths = [...selected];
    if (!paths.length) return;
    try {
      setError("");
      await bulkDownload(paths);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha no download");
    }
  }

  async function onBulkRestore() {
    const paths = [...selected];
    if (!paths.length) return;
    try {
      await bulkRestore(paths);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao restaurar");
    }
  }

  async function onBulkPurge() {
    const paths = [...selected];
    if (!paths.length) return;
    if (!window.confirm(`Excluir definitivamente ${paths.length} item(ns)?`)) return;
    try {
      await bulkPurge(paths);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao excluir");
    }
  }

  const crumbs = path ? path.split("/") : [];
  const openAlbum = galleryAlbumPath
    ? galleryAlbums.find((a) => a.path === galleryAlbumPath)
    : undefined;
  const title =
    view === "gallery"
      ? galleryAlbumPath
        ? openAlbum?.name || galleryAlbumPath.split("/").pop() || "Galeria"
        : "Galeria"
      : view === "trash"
        ? "Lixeira"
        : path
          ? crumbs[crumbs.length - 1]
          : "Meus arquivos";
  const selectedCount = selected.size;
  const allFilteredSelected = filtered.length > 0 && filtered.every((f) => selected.has(f.path));

  return (
    <div className="shell">
      <header className="topbar">
        <div className="logo">
          <span className="logo-mark">N</span>
          NetoDrive
        </div>
        <div className="search">
          <input
            placeholder="Pesquisar arquivos"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
        <div className="user">
          <button className="btn" onClick={onLogout}>
            Sair
          </button>
        </div>
      </header>

      <aside className="sidebar">
        <button
          type="button"
          className={`nav-item ${view === "files" ? "active" : ""}`}
          onClick={() => {
            setView("files");
            setPath("");
            setPreview(null);
          }}
        >
          <span className="nav-icon">Fs</span> Meus arquivos
        </button>
        <button
          type="button"
          className={`nav-item ${view === "gallery" ? "active" : ""}`}
          onClick={() => {
            setView("gallery");
            setPreview(null);
          }}
        >
          <span className="nav-icon">Gl</span> Galeria
        </button>
        <button
          type="button"
          className={`nav-item ${view === "trash" ? "active" : ""}`}
          onClick={() => {
            setView("trash");
            setPreview(null);
          }}
        >
          <span className="nav-icon">Lz</span> Lixeira
        </button>
      </aside>

      <main className="main">
        <div className="page-header">
          <h2>{title}</h2>
          {view === "files" ? (
            <nav className="breadcrumb">
              <button type="button" onClick={() => refresh("")}>
                NetoDrive
              </button>
              {crumbs.map((part, i) => {
                const full = crumbs.slice(0, i + 1).join("/");
                return (
                  <span key={full}>
                    {" › "}
                    <button type="button" onClick={() => refresh(full)}>
                      {part}
                    </button>
                  </span>
                );
              })}
            </nav>
          ) : view === "trash" ? (
            <p className="muted" style={{ margin: "0.35rem 0 0" }}>
              Itens excluidos. Restaure ou apague definitivamente.
            </p>
          ) : (
            <p className="muted" style={{ margin: "0.35rem 0 0" }}>
              {galleryAlbumPath
                ? "Mídia deste álbum"
                : "Álbuns da pasta Galeria — fotos e vídeos sincronizados"}
            </p>
          )}
        </div>

        <div className="command-bar">
          {view === "files" ? (
            <>
              <label className="btn">
                + Novo / Enviar
                <input
                  type="file"
                  multiple
                  hidden
                  onChange={(e) => {
                    void onUpload(e.target.files);
                    e.target.value = "";
                  }}
                />
              </label>
              <button className="btn secondary" onClick={onMkdir}>
                Nova pasta
              </button>
            </>
          ) : null}
          {view === "gallery" && galleryAlbumPath ? (
            <button className="btn secondary" onClick={loadGalleryAlbums}>
              ← Voltar aos álbuns
            </button>
          ) : null}
          {view === "trash" ? (
            <button className="btn danger" onClick={() => void onEmptyTrash()}>
              Esvaziar lixeira
            </button>
          ) : null}
          {selectedCount > 0 && view !== "gallery" ? (
            <div className="bulk-bar">
              <span className="muted">{selectedCount} selecionado(s)</span>
              {view === "trash" ? (
                <>
                  <button className="btn secondary" onClick={() => void onBulkRestore()}>
                    Restaurar
                  </button>
                  <button className="btn danger" onClick={() => void onBulkPurge()}>
                    Excluir definitivo
                  </button>
                </>
              ) : (
                <>
                  <button className="btn secondary" onClick={() => void onBulkDownload()}>
                    Baixar ZIP
                  </button>
                  <button className="btn danger" onClick={() => void onBulkDelete()}>
                    Excluir
                  </button>
                </>
              )}
              <button className="btn ghost" onClick={clearSelection}>
                Limpar
              </button>
            </div>
          ) : null}
          <button className="btn secondary" onClick={() => refresh(path)} disabled={pending}>
            Atualizar
          </button>
          {error ? <span className="error">{error}</span> : null}
        </div>

        {view === "gallery" ? (
          <div className="gallery-pane">
            {galleryAlbumPath === null ? (
              filteredAlbums.length === 0 ? (
                <div className="empty">
                  <h3>{pending ? "Carregando…" : "Nenhum álbum"}</h3>
                  <p>
                    Crie pastas dentro de Galeria (na árvore de arquivos) ou sincronize a galeria no
                    Android.
                  </p>
                </div>
              ) : (
                <div className="album-grid">
                  {filteredAlbums.map((album) => (
                    <button
                      key={album.path}
                      type="button"
                      className="album-card"
                      onClick={() => openGalleryAlbum(album.path)}
                    >
                      <div className="album-cover">
                        {album.cover ? (
                          <img src={openUrl(album.cover)} alt="" />
                        ) : (
                          <span className="album-cover-fallback">Álbum</span>
                        )}
                      </div>
                      <div className="album-meta">
                        <strong>{album.name}</strong>
                        <span className="muted">
                          {album.count} {album.count === 1 ? "item" : "itens"}
                        </span>
                      </div>
                    </button>
                  ))}
                </div>
              )
            ) : filteredGalleryItems.length === 0 ? (
              <div className="empty">
                <h3>{pending ? "Carregando…" : "Álbum vazio"}</h3>
                <p>Não há fotos ou vídeos neste álbum.</p>
              </div>
            ) : (
              <div className="media-grid">
                {filteredGalleryItems.map((item) => {
                  const isVideo = item.mime.startsWith("video/");
                  return (
                    <button
                      key={item.id}
                      type="button"
                      className="media-thumb"
                      onClick={() =>
                        setPreview({ path: item.path, name: item.name, mime: item.mime })
                      }
                      title={item.name}
                    >
                      {isVideo ? (
                        <video src={openUrl(item.path)} muted preload="metadata" />
                      ) : (
                        <img src={openUrl(item.path)} alt={item.name} loading="lazy" />
                      )}
                      {isVideo ? <span className="media-badge">VID</span> : null}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        ) : (
          <div className="file-table">
            {filtered.length === 0 ? (
              <div className="empty">
                <h3>
                  {pending ? "Carregando…" : view === "trash" ? "Lixeira vazia" : "Esta pasta está vazia"}
                </h3>
                <p>
                  {view === "trash"
                    ? "Arquivos excluidos aparecem aqui ate a exclusao definitiva."
                    : "Envie arquivos pela web, pelo cliente Windows ou sincronize a galeria no Android."}
                </p>
              </div>
            ) : (
              <table>
                <thead>
                  <tr>
                    <th className="col-check">
                      <input
                        type="checkbox"
                        checked={allFilteredSelected}
                        onChange={() => toggleSelectAll(filtered)}
                        aria-label="Selecionar todos"
                      />
                    </th>
                    <th>Nome</th>
                    <th>Modificado</th>
                    <th>Tamanho</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((f) => (
                    <tr key={f.id} className={selected.has(f.path) ? "selected" : undefined}>
                      <td className="col-check">
                        <input
                          type="checkbox"
                          checked={selected.has(f.path)}
                          onChange={() => toggleSelect(f.path)}
                          aria-label={`Selecionar ${f.name}`}
                        />
                      </td>
                      <td>
                        <div className="file-name-cell">
                          <span className={`icon-tile ${tileClass(f)}`}>{tileLabel(f)}</span>
                          {view !== "trash" && f.is_dir ? (
                            <button type="button" className="linkish" onClick={() => refresh(f.path)}>
                              {f.name}
                            </button>
                          ) : view !== "trash" && !f.is_dir ? (
                            <button
                              type="button"
                              className="linkish"
                              onClick={() => setPreview({ path: f.path, name: f.name, mime: f.mime })}
                            >
                              {f.name}
                            </button>
                          ) : (
                            <span>{f.name}</span>
                          )}
                        </div>
                      </td>
                      <td className="muted">{formatDate(f.mtime)}</td>
                      <td className="muted">{f.is_dir ? "—" : formatSize(f.size)}</td>
                      <td>
                        <div className="row-actions">
                          {view === "trash" ? (
                            <>
                              <button className="btn ghost" onClick={() => void onRestore(f)}>
                                Restaurar
                              </button>
                              <button className="btn ghost" onClick={() => void onPurge(f)}>
                                Excluir definitivo
                              </button>
                            </>
                          ) : (
                            <>
                              {!f.is_dir ? (
                                <>
                                  <button
                                    className="btn ghost"
                                    onClick={() =>
                                      setPreview({ path: f.path, name: f.name, mime: f.mime })
                                    }
                                  >
                                    Abrir
                                  </button>
                                  <a className="btn ghost" href={downloadUrl(f.path)} download={f.name}>
                                    Baixar
                                  </a>
                                </>
                              ) : null}
                              <button className="btn ghost" onClick={() => void onDelete(f)}>
                                Excluir
                              </button>
                            </>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}

        {preview ? <RemotePreview preview={preview} onClose={() => setPreview(null)} /> : null}
      </main>
    </div>
  );
}

function RemotePreview({ preview, onClose }: { preview: Preview; onClose: () => void }) {
  const url = openUrl(preview.path);
  const mime = preview.mime;
  return (
    <section className="preview-dock">
      <header>
        <strong>{preview.name}</strong>
        <button className="btn secondary" onClick={onClose}>
          Fechar
        </button>
      </header>
      {mime.startsWith("image/") ? (
        <img src={url} alt={preview.name} />
      ) : mime.startsWith("video/") ? (
        <video src={url} controls />
      ) : mime.startsWith("audio/") ? (
        <audio src={url} controls style={{ width: "100%" }} />
      ) : mime === "application/pdf" ? (
        <iframe title={preview.name} src={url} />
      ) : mime.startsWith("text/") || mime === "application/json" ? (
        <TextPreview url={url} />
      ) : (
        <p className="muted">
          Sem pré-visualização.{" "}
          <a href={url} target="_blank" rel="noreferrer">
            Abrir remoto
          </a>
        </p>
      )}
    </section>
  );
}

function TextPreview({ url }: { url: string }) {
  const [text, setText] = useState("Carregando…");
  useEffect(() => {
    fetch(url)
      .then((r) => r.text())
      .then(setText)
      .catch(() => setText("Não foi possível carregar o conteúdo."));
  }, [url]);
  return <pre>{text}</pre>;
}

function tileClass(f: FileMeta) {
  if (f.is_dir) return "folder";
  if (f.mime.startsWith("image/")) return "image";
  if (f.mime.startsWith("video/")) return "video";
  return "file";
}

function tileLabel(f: FileMeta) {
  if (f.is_dir) return "DIR";
  if (f.mime.startsWith("image/")) return "IMG";
  if (f.mime.startsWith("video/")) return "VID";
  const ext = f.name.includes(".") ? f.name.split(".").pop()!.slice(0, 3).toUpperCase() : "DOC";
  return ext;
}

function formatDate(raw: string) {
  if (!raw) return "—";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
}
