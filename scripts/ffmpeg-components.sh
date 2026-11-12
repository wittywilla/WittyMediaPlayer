#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 WittyWilla and contributors
# What the player's own FFmpeg is built with, the same on every system: the decoders,
# demuxers, parsers and bitstream filters a player needs, and nothing that reaches the
# network or a disc-decryption library. scripts/setup-toolchain.sh (Windows) and
# scripts/setup-toolchain-linux.sh source this; the packages carry it beside FFmpeg's source.
DEC="mpeg1video mpeg2video mpeg4 h263 h264 hevc vc1 wmv1 wmv2 wmv3 vp8 vp9 av1 theora msmpeg4v1 msmpeg4v2 msmpeg4v3 mjpeg
     aac aac_latm ac3 eac3 dca truehd mlp mp1 mp2 mp3 mp3float flac alac opus vorbis wmav1 wmav2 wmapro wmalossless
     pcm_s8 pcm_u8 pcm_s16le pcm_s16be pcm_s24le pcm_s24be pcm_s32le pcm_s32be pcm_f32le pcm_f64le pcm_alaw pcm_mulaw pcm_dvd pcm_bluray adpcm_ms ape tta wavpack
     dvdsub pgssub dvbsub xsub subrip srt ass ssa webvtt text mov_text"
DMX="matroska mov avi mpegts mpegps mpegvideo mpegtsraw asf flv ogg wav flac mp3 aac ac3 eac3 dts dtshd truehd mlp ape wv aiff tta caf nut rm vc1 h264 hevc m4v
     srt ass webvtt"
PAR="h264 hevc mpegvideo mpeg4video vc1 vp8 vp9 av1 aac aac_latm ac3 dca mlp mpegaudio flac opus vorbis dvdsub dvd_nav dvbsub pgs"
BSF="aac_adtstoasc h264_mp4toannexb hevc_mp4toannexb extract_extradata vp9_superframe_split vp9_superframe av1_frame_merge av1_frame_split dts2pts truehd_core"
ENC="flac,alac,aac,pcm_s16le,libmp3lame,libopus,libvorbis"
MUX="flac,wav,ipod,mp4,mp3,ogg,opus"
FILTERS="yadif,bwdif,buffer,buffersink,format,scale"
list() { echo $1 | tr ' ' ','; }
