package com.netodrive.app.ui

import android.graphics.Bitmap
import android.view.LayoutInflater
import android.view.ViewGroup
import android.widget.ImageView
import androidx.recyclerview.widget.RecyclerView
import com.netodrive.app.api.FileMeta
import com.netodrive.app.databinding.ItemGalleryBinding

class GalleryAdapter(
    private val onOpen: (FileMeta) -> Unit,
    private val thumbLoader: (FileMeta, ImageView) -> Unit,
) : RecyclerView.Adapter<GalleryAdapter.VH>() {
    private val items = mutableListOf<FileMeta>()

    fun submit(list: List<FileMeta>) {
        items.clear()
        items.addAll(list)
        notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): VH {
        val binding = ItemGalleryBinding.inflate(LayoutInflater.from(parent.context), parent, false)
        return VH(binding)
    }

    override fun getItemCount(): Int = items.size

    override fun onBindViewHolder(holder: VH, position: Int) {
        val item = items[position]
        holder.binding.title.text = item.name
        holder.binding.subtitle.text = "${item.mime} · ${(item.size / 1024)} KB"
        holder.binding.thumb.setImageBitmap(null as Bitmap?)
        thumbLoader(item, holder.binding.thumb)
        holder.binding.openBtn.setOnClickListener { onOpen(item) }
        holder.itemView.setOnClickListener { onOpen(item) }
    }

    class VH(val binding: ItemGalleryBinding) : RecyclerView.ViewHolder(binding.root)
}
