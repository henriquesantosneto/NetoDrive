import { FormEvent, useEffect, useState, useTransition } from "react";
import {
  clearToken,
  createDir,
  deleteFile,
  downloadUrl,
  FileMeta,
  formatSize,
  getToken,
  listFiles,
  login,
  openUrl,
  setToken,
  uploadFile,
} from "./api";

type Preview = {
  path: string;
  name: string;
  mime: string;
};

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
        <h1>
          Neto<span>Drive</span>
        </h1>
        <p className="muted">Seus arquivos em Windows, Android e na web — um só servidor Linux.</p>
        <form onSubmit={submit}>
          <label>
            Usuário
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
  const [path, setPath] = useState("");
  const [files, setFiles] = useState<FileMeta[]>([]);
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<Preview | null>(null);
  const [pending, start] = useTransition();

  function refresh(nextPath = path) {
    start(async () => {
      try {
        setError("");
        const res = await listFiles(nextPath);
        setFiles(res.files);
        setPath(nextPath);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Erro ao listar");
      }
    });
  }

  useEffect(() => {
    refresh("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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
    if (!window.confirm(`Apagar ${f.name}?`)) return;
    try {
      await deleteFile(f.path);
      if (preview?.path === f.path) setPreview(null);
      refresh(path);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao apagar");
    }
  }

  const crumbs = path ? path.split("/") : [];

  return (
    <div className="app-shell">
      <header className="topbar">
        <div>
          <h1 className="brand">
            Neto<span>Drive</span>
          </h1>
          <nav className="crumb">
            <button type="button" onClick={() => refresh("")}>
              raiz
            </button>
            {crumbs.map((part, i) => {
              const full = crumbs.slice(0, i + 1).join("/");
              return (
                <span key={full}>
                  /{" "}
                  <button type="button" onClick={() => refresh(full)}>
                    {part}
                  </button>
                </span>
              );
            })}
          </nav>
        </div>
        <button className="btn secondary" onClick={onLogout}>
          Sair
        </button>
      </header>

      <div className="toolbar">
        <label className="btn">
          Enviar
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
        <button className="btn secondary" onClick={() => refresh(path)} disabled={pending}>
          Atualizar
        </button>
      </div>

      {error ? <p className="error">{error}</p> : null}

      <div className="file-list">
        {files.length === 0 ? (
          <div className="empty">{pending ? "Carregando…" : "Pasta vazia — envie arquivos ou sincronize um cliente."}</div>
        ) : (
          files.map((f) => (
            <div className="file-row" key={f.id}>
              <div className="file-main">
                <div className="file-icon">{f.is_dir ? "DIR" : extLabel(f.name)}</div>
                <div style={{ minWidth: 0 }}>
                  <div className="file-name">
                    {f.is_dir ? (
                      <button
                        type="button"
                        className="btn secondary"
                        style={{ padding: "0.15rem 0.5rem" }}
                        onClick={() => refresh(f.path)}
                      >
                        {f.name}
                      </button>
                    ) : (
                      f.name
                    )}
                  </div>
                  <div className="file-meta">
                    {f.is_dir ? "pasta" : `${formatSize(f.size)} · ${f.mime}`}
                  </div>
                </div>
              </div>
              <div className="file-actions">
                {!f.is_dir ? (
                  <>
                    <button
                      className="btn secondary"
                      onClick={() => setPreview({ path: f.path, name: f.name, mime: f.mime })}
                    >
                      Abrir
                    </button>
                    <a className="btn secondary" href={downloadUrl(f.path)} download={f.name}>
                      Baixar
                    </a>
                  </>
                ) : null}
                <button className="btn danger" onClick={() => void onDelete(f)}>
                  Apagar
                </button>
              </div>
            </div>
          ))
        )}
      </div>

      {preview ? <RemotePreview preview={preview} onClose={() => setPreview(null)} /> : null}
    </div>
  );
}

function RemotePreview({ preview, onClose }: { preview: Preview; onClose: () => void }) {
  const url = openUrl(preview.path);
  const mime = preview.mime;

  return (
    <section className="preview">
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
          Pré-visualização indisponível.{" "}
          <a href={url} target="_blank" rel="noreferrer">
            Abrir remoto
          </a>{" "}
          ou baixar o arquivo.
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

function extLabel(name: string) {
  const ext = name.includes(".") ? name.split(".").pop()!.slice(0, 3).toUpperCase() : "FILE";
  return ext;
}
