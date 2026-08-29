package com.netodrive.app.ui

import android.view.LayoutInflater
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView
import com.netodrive.app.api.FileMeta
import com.netodrive.app.databinding.ItemFileBinding

class FileListAdapter(
    private val onClick: (FileMeta) -> Unit,
    private val onLongClick: (FileMeta) -> Unit,
    private val isPinned: (String) -> Boolean,
    private val isLocal: (FileMeta) -> Boolean,
) : RecyclerView.Adapter<FileListAdapter.VH>() {
    private val items = mutableListOf<FileMeta>()

    fun submit(list: List<FileMeta>) {
        items.clear()
        items.addAll(list)
        notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): VH {
        val binding = ItemFileBinding.inflate(LayoutInflater.from(parent.context), parent, false)
        return VH(binding)
    }

    override fun getItemCount(): Int = items.size

    override fun onBindViewHolder(holder: VH, position: Int) {
        val item = items[position]
        holder.binding.title.text = item.name
        val state = when {
            item.isDir && isPinned(item.path) -> "Pasta · fixada neste aparelho"
            item.isDir -> "Pasta · na nuvem"
            isPinned(item.path) || isLocal(item) -> "${item.mime} · neste aparelho"
            else -> "${item.mime} · nuvem (toque para baixar)"
        }
        holder.binding.subtitle.text = if (item.isDir) {
            state
        } else {
            "$state · ${item.size / 1024} KB"
        }
        holder.binding.icon.text = when {
            item.isDir && isPinned(item.path) -> "PIN"
            item.isDir -> "DIR"
            isPinned(item.path) || isLocal(item) -> "LOC"
            item.mime.startsWith("image/") -> "CLD"
            item.mime.startsWith("video/") -> "CLD"
            else -> "CLD"
        }
        holder.itemView.setOnClickListener { onClick(item) }
        holder.itemView.setOnLongClickListener {
            onLongClick(item)
            true
        }
    }

    class VH(val binding: ItemFileBinding) : RecyclerView.ViewHolder(binding.root)
}
