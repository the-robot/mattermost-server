// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type sqlRemoteClusterStore struct {
	SqlStore
}

func newSqlRemoteClustersStore(sqlStore SqlStore) store.RemoteClusterStore {
	s := &sqlRemoteClusterStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.RemoteCluster{}, "RemoteClusters").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("ClusterName").SetMaxSize(64)
		table.ColMap("Hostname").SetMaxSize(512)
		table.ColMap("Token").SetMaxSize(26)
		table.ColMap("Topics").SetMaxSize(512)
	}
	return s
}

func (s sqlRemoteClusterStore) Save(remoteCluster *model.RemoteCluster) (*model.RemoteCluster, error) {
	remoteCluster.PreSave()
	if err := remoteCluster.IsValid(); err != nil {
		return nil, err
	}

	if err := s.GetMaster().Insert(remoteCluster); err != nil {
		return nil, errors.Wrap(err, "failed to save RemoteCluster")
	}
	return remoteCluster, nil
}

func (s sqlRemoteClusterStore) Delete(remoteClusterId string) (bool, error) {
	squery, args, err := s.getQueryBuilder().
		Delete("RemoteClusters").
		Where(sq.Eq{"Id": remoteClusterId}).
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "delete_remote_cluster_tosql")
	}

	result, err := s.GetMaster().Exec(squery, args...)
	if err != nil {
		return false, errors.Wrap(err, "failed to delete RemoteCluster")
	}

	count, err := result.RowsAffected()
	if err != nil {
		return false, errors.Wrap(err, "failed to determine rows affected")
	}

	return count > 0, nil
}

func (s sqlRemoteClusterStore) Get(remoteClusterId string) (*model.RemoteCluster, error) {
	query := s.getQueryBuilder().
		Select("*").
		From("RemoteClusters").
		Where(sq.Eq{"Id": remoteClusterId})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "remote_cluster_get_tosql")
	}

	var rc model.RemoteCluster
	if err := s.GetReplica().SelectOne(&rc, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find RemoteCluster")
	}
	return &rc, nil
}

func (s sqlRemoteClusterStore) GetAll(includeOffline bool) ([]*model.RemoteCluster, error) {
	query := s.getQueryBuilder().
		Select("*").
		From("RemoteClusters")

	if !includeOffline {
		query = query.Where(sq.Gt{"LastPingAt": model.GetMillis() - model.RemoteOfflineAfterMillis})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "remote_cluster_getall_tosql")
	}

	var list []*model.RemoteCluster
	if _, err := s.GetReplica().Select(&list, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find RemoteCluster")
	}
	return list, nil
}

func (s sqlRemoteClusterStore) GetAllNotInChannel(channelId string, inclOffline bool) ([]*model.RemoteCluster, error) {
	query := s.getQueryBuilder().
		Select("rc.*").
		From("RemoteClusters rc").
		Where("rc.Id NOT IN (SELECT scr.RemoteClusterId FROM SharedChannelRemotes scr WHERE scr.ChannelId = ?)", channelId)

	if !inclOffline {
		query = query.Where(sq.Gt{"rc.LastPingAt": model.GetMillis() - model.RemoteOfflineAfterMillis})
	}

	queryString, args, err := query.OrderBy("rc.ClusterName ASC").ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "remote_cluster_getallnotinchannel_tosql")
	}

	var list []*model.RemoteCluster
	if _, err := s.GetReplica().Select(&list, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find RemoteCluster")
	}
	return list, nil
}

func (s sqlRemoteClusterStore) GetByTopic(topic string) ([]*model.RemoteCluster, error) {
	trimmed := strings.TrimSpace(topic)
	if trimmed == "" || trimmed == "*" {
		return nil, errors.New("invalid topic")
	}

	queryTopic := fmt.Sprintf("%% %s %%", trimmed)
	query := s.getQueryBuilder().
		Select("rc.*").
		From("RemoteClusters rc").
		Where(sq.Or{sq.Like{"rc.Topics": queryTopic}, sq.Eq{"rc.Topics": "*"}})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "remote_cluster_getbytopic_tosql")
	}

	var list []*model.RemoteCluster
	if _, err := s.GetReplica().Select(&list, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find RemoteCluster")
	}
	return list, nil
}

func (s sqlRemoteClusterStore) UpdateTopics(remoteClusterid string, topics string) (*model.RemoteCluster, error) {
	rc, err := s.Get(remoteClusterid)
	if err != nil {
		return nil, err
	}
	rc.Topics = topics

	rc.PreUpdate()

	if _, err = s.GetMaster().Update(rc); err != nil {
		return nil, err
	}
	return rc, nil
}

func (s sqlRemoteClusterStore) SetLastPingAt(remoteClusterId string) error {
	query := s.getQueryBuilder().
		Update("RemoteClusters").
		Set("LastPingAt", model.GetMillis()).
		Where(sq.Eq{"Id": remoteClusterId})

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "remote_cluster_tosql")
	}

	if _, err := s.GetMaster().Exec(queryString, args...); err != nil {
		return errors.Wrap(err, "failed to update RemoteCluster")
	}
	return nil
}
