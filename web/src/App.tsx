import { FormEvent, useEffect, useMemo, useState, useTransition } from "react";
import {
  clearToken,
  createDir,
  deleteFile,
  downloadUrl,
  emptyTrash,
  FileMeta,
  formatSize,
  getToken,
  listFiles,
  listGallery,
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

  function refresh(nextPath = path) {
    start(async () => {
      try {
        setError("");
        if (view === "gallery") {
          const res = await listGallery(120, 0);
          setFiles(res.items);
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
    refresh(view === "files" ? path : "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return files;
    return files.filter((f) => f.name.toLowerCase().includes(q) || f.path.toLowerCase().includes(q));
  }, [files, query]);

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

  const crumbs = path ? path.split("/") : [];
  const title =
    view === "gallery" ? "Galeria" : view === "trash" ? "Lixeira" : path ? crumbs[crumbs.length - 1] : "Meus arquivos";

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
          className={`nav-item ${view === "files" && !path.startsWith("Gallery") ? "active" : ""}`}
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
          className={`nav-item ${path === "PC" ? "active" : ""}`}
          onClick={() => {
            setView("files");
            setPath("PC");
            setPreview(null);
            start(async () => {
              try {
                const res = await listFiles("PC");
                setFiles(res.files);
              } catch (err) {
                setError(err instanceof Error ? err.message : "Erro");
              }
            });
          }}
        >
          <span className="nav-icon">PC</span> Este computador
        </button>
        <button
          type="button"
          className="nav-item"
          onClick={() => {
            setView("files");
            setPath("Gallery");
            setPreview(null);
            start(async () => {
              try {
                const res = await listFiles("Gallery");
                setFiles(res.files);
              } catch (err) {
                setError(err instanceof Error ? err.message : "Erro");
              }
            });
          }}
        >
          <span className="nav-icon">And</span> Do Android
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
              Fotos sincronizadas dos dispositivos (modo cache no Android)
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
          {view === "trash" ? (
            <button className="btn danger" onClick={() => void onEmptyTrash()}>
              Esvaziar lixeira
            </button>
          ) : null}
          <button className="btn secondary" onClick={() => refresh(path)} disabled={pending}>
            Atualizar
          </button>
          {error ? <span className="error">{error}</span> : null}
        </div>

        <div className="file-table">
          {filtered.length === 0 ? (
            <div className="empty">
              <h3>{pending ? "Carregando…" : view === "trash" ? "Lixeira vazia" : "Esta pasta está vazia"}</h3>
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
                  <th>Nome</th>
                  <th>Modificado</th>
                  <th>Tamanho</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((f) => (
                  <tr key={f.id}>
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
                                  onClick={() => setPreview({ path: f.path, name: f.name, mime: f.mime })}
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
