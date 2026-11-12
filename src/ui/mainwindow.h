// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
// The player's window.
#pragma once

#include <QJsonArray>
#include <QJsonObject>
#include <QMap>
#include <QPointer>
#include <QMainWindow>
#include <QSettings>
#include <QTimer>
#include <QVector>
#include <functional>
#include <memory>

#include "../disc/discmemory.h"

class QAbstractButton;
class QAction;
class QFrame;
class QLabel;
class QMenu;
class QVBoxLayout;

namespace wv {
class VisualizerWidget;
}

namespace wmp {

class CdPage;
class ControlBar;
class DiscController;
class EngineClient;
class MessageOverlay;
class Player;
class VideoView;
class PageHost;
class PageDialog;
class SduiView;

class MainWindow : public QMainWindow {
    Q_OBJECT
public:
    explicit MainWindow(bool noAudioDevice = false, QWidget *parent = nullptr);
    ~MainWindow() override;

    // Opens a file, a disc image, a disc folder or a drive.
    void openPath(const QString &path);

    Player *player() const { return player_; }
    DiscController *disc() const { return disc_; }
    CdPage *cd() const { return cd_; }
    VideoView *view() const { return view_; }
    wv::VisualizerWidget *visualizer() const { return viz_; }
    bool visualizerShowing() const;
    EngineClient *engine() const { return engine_; }

    // What's on show, for tests and the window's own status line.
    QString statusText() const { return status_; }
    QString shownMessage() const;
    QString shownSource() const { return currentPath_; }
    int menuButtonCount() const;

    // Handles a key as the window would. Returns false if it isn't one of its keys.
    bool handleKey(int key, Qt::KeyboardModifiers mods = Qt::NoModifier);

    void showMessage(const QString &text, bool stopsPlayback = false);
    // A short note over the picture, such as the track just chosen, that goes by itself.
    void showNotice(const QString &text);
    QString shownNotice() const;

    // Which libraries play the next thing opened: the viewer's choice, from Settings.
    void setUseVlc(bool on);
    // Draws a page at 1000 by 760 off screen and saves it as a picture, for screenshots.
    void grabPage(const QString &page, const QString &file);
    PageHost *pages() const { return pages_; }

protected:
    void closeEvent(QCloseEvent *e) override;
    void dragEnterEvent(QDragEnterEvent *e) override;
    void dropEvent(QDropEvent *e) override;
    bool eventFilter(QObject *o, QEvent *e) override;
    void resizeEvent(QResizeEvent *e) override;
    void mouseMoveEvent(QMouseEvent *e) override;
    bool nativeEvent(const QByteArray &type, void *message, qintptr *result) override;

private:
    void buildMenus();
    void buildStartPage();
    void refreshDrives();
    // The screens that are SDUI pages (src/sdui, engine/internal/pages): what the engine is
    // told about the window, what an action on one does, and where one is shown.
    QJsonObject pageState();
    void handleAction(SduiView *from, const QJsonObject &action);
    void showPage(const QString &page, bool firstRun = false);
    void reloadPages();
    void runUpdater(const QStringList &args, const std::function<void(const QJsonObject &)> &done);
    void updateVlcStatus();
    bool canMakeMain() const;
    void chooseTheme(const QString &id);
    void showRemote();
    void pressRemoteKey(const QString &key);
    void rebuildTrackMenus();
    void rebuildDiscMenus();
    void showWhy();
    void updateFeatureActions();
    void rememberFeature(const DiscPick &pick);
    void makeThisTheFeature();
    void forgetChosenFeature();
    void rebuildRecent();
    void rememberRecent(const QString &path);
    void updateHitButtons();
    void layoutOverlays();
    void onTick();
    void togglePlay();
    void setFullscreen(bool on);
    void showControls();
    void openDialog();
    void openFolderDialog();
    void makeImage(const QString &driveOrImage = QString());
    void openSettings();
    void applyEngine();
    void lookForModuleUpdates();
    void stopPlayback(bool showStart = true);
    void setStatus(const QString &text);
    void setVolumePercent(int percent);
    void cycleAudio();
    // Music only: on a video it would hide the picture. Made the first time it's wanted.
    void setVisualizer(bool on);
    void cycleSubtitle();
    void updateFileSubtitles();
    bool cdOn() const;
    void showCd(const QJsonObject &info);
    void leaveCd();

    Player *player_ = nullptr;
    VideoView *view_ = nullptr;
    wv::VisualizerWidget *viz_ = nullptr;
    QAction *actViz_ = nullptr;
    EngineClient *engine_ = nullptr;
    DiscController *disc_ = nullptr;
    CdPage *cd_ = nullptr;
    ControlBar *bar_ = nullptr;
    QWidget *central_ = nullptr;
    SduiView *start_ = nullptr;
    PageHost *pages_ = nullptr;
    QPointer<PageDialog> dialog_, remote_;
    QMap<QString, QString> offers_;
    QString moduleNote_, vlcStatus_, startNotice_, themeNote_;
    bool firstRun_ = false;
    int remotePage_ = 0;
    QLabel *announcer_ = nullptr;
    MessageOverlay *message_ = nullptr;
    QLabel *audioCaption_ = nullptr;
    QLabel *notice_ = nullptr;
    QVector<QAbstractButton *> hits_;
    QMenu *menuAudio_ = nullptr, *menuSubs_ = nullptr, *menuDisc_ = nullptr, *menuTitles_ = nullptr, *menuChapters_ = nullptr, *menuAngles_ = nullptr, *menuVersions_ = nullptr, *menuEpisodes_ = nullptr, *menuRecent_ = nullptr, *menuDrives_ = nullptr;
    QAction *actStop_ = nullptr, *actGuide_ = nullptr, *actHealth_ = nullptr, *actMain_ = nullptr, *actWhy_ = nullptr, *actMakeMain_ = nullptr, *actForgetMain_ = nullptr, *actImage_ = nullptr, *actLocks_ = nullptr, *actForced_ = nullptr, *actFull_ = nullptr;
    QTimer tick_, hideControls_, driveWatch_, noticeTimer_;
    QSettings settings_;
    QString currentPath_;
    QString status_;
    QString pendingDrivePrompt_;
    bool isFile_ = false;
    bool fullscreen_ = false;
    bool muted_ = false;
    int volume_ = 100;
    bool ended_ = false;
    uint64_t subSig_ = 0;
    QVector<int> audioIds_, subIds_;
    QVector<bool> subExternal_;
};

} // namespace wmp
