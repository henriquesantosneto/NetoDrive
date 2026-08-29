package com.netodrive.app.ui

import android.view.LayoutInflater
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView
import com.netodrive.app.api.FileMeta
import com.netodrive.app.databinding.ItemFileBinding

class FileListAdapter(
    private val onClick: (FileMeta) -> Unit,
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
        holder.binding.subtitle.text = if (item.isDir) {
            "Pasta"
        } else {
            "${item.mime} · ${item.size / 1024} KB"
        }
        holder.binding.icon.text = when {
            item.isDir -> "DIR"
            item.mime.startsWith("image/") -> "IMG"
            item.mime.startsWith("video/") -> "VID"
            else -> "DOC"
        }
        holder.itemView.setOnClickListener { onClick(item) }
    }

    class VH(val binding: ItemFileBinding) : RecyclerView.ViewHolder(binding.root)
}
