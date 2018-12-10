package state

import (
	"fmt"
	"sync"

	"github.com/VivaLaPanda/antipath/entity"
	"github.com/VivaLaPanda/antipath/state/tile"
	uuid "github.com/satori/go.uuid"
)

type State struct {
	grid         [][]tile.Tile
	size         int
	entities     map[EntityID]Coordinates
	entitiesLock *sync.RWMutex
}

type Coordinates struct {
	X, Y int
}

type Direction int
type EntityID string

const (
	MovUp    Direction = iota
	MovRight Direction = iota
	MovLeft  Direction = iota
	MovDown  Direction = iota
	MovNone  Direction = iota
)

func NewState(size int) (grid *State) {
	if size < 1 {
		panic("state must be at least 1x1 size")
	}
	gridData := make([][]tile.Tile, size)
	for idx := range gridData {
		gridData[idx] = make([]tile.Tile, size)
	}
	return &State{
		grid:         gridData,
		size:         size, // faster than using len every time
		entities:     make(map[EntityID]Coordinates),
		entitiesLock: &sync.RWMutex{},
	}
}

func (s *State) Size() int {
	return s.size
}

func (s *State) GetTile(pos Coordinates) (*tile.Tile, error) {
	if outOfBounds(s.size, pos) {
		return nil, fmt.Errorf("provided pos is out of bounds. Pos: %v, maxsize: %d", pos, s.size)
	}
	return &s.grid[pos.Y][pos.X], nil
}

func (s *State) NewEntity(data entity.Entity, pos Coordinates) (id EntityID, err error) {
	targetTile, err := s.GetTile(pos)
	if err != nil {
		return "", err
	}

	if err := targetTile.SetEntity(data); err != nil {
		return "", fmt.Errorf("provided pos can't contain an entity, already full. Tile %v", targetTile)
	}

	id = EntityID(uuid.Must(uuid.NewV4()).String())

	s.entitiesLock.Lock()
	s.entities[id] = pos
	s.entitiesLock.Unlock()

	return id, nil
}

func (s *State) GetEntityPos(entityID EntityID) (pos Coordinates, exists bool) {
	s.entitiesLock.RLock()
	pos, exists = s.entities[entityID]
	s.entitiesLock.RUnlock()
	return
}

func (s *State) PeekState(entityID EntityID) [][]tile.Tile {
	windowSize := 10
	// Expand a window around the entity
	s.entitiesLock.RLock()
	pos := s.entities[entityID]
	s.entitiesLock.RUnlock()
	minX := forceBounds(pos.X-windowSize, s.size)
	minY := forceBounds(pos.Y-windowSize, s.size)
	maxX := forceBounds(pos.X+windowSize, s.size)
	maxY := forceBounds(pos.Y+windowSize, s.size)

	// Grab the part of the grid described by the bounds above
	ySlice := s.grid[minY:maxY]
	gridCopy := make([][]tile.Tile, len(ySlice))
	for idx, row := range ySlice {
		gridCopy[idx] = row[minX:maxX]
	}

	return gridCopy
}

func (s *State) Move(entityID EntityID, dir Direction, speed int, altitude int) (err error) {
	// Get the location of the entity
	s.entitiesLock.RLock()
	sourcePos, exists := s.entities[entityID]
	s.entitiesLock.RUnlock()
	if !exists {
		return fmt.Errorf("provided entity ID not valid. ID: %s", entityID)
	}
	// Get the tile data at that location
	sourceTile, err := s.GetTile(sourcePos)
	if err != nil {
		return fmt.Errorf("couldn't get tile at provided pos, pos: %v, err: %s", sourcePos, err)
	}

	// Calculate the total movement
	targetPos := sourcePos
	var targetTile *tile.Tile
	switch dir {
	case MovUp:
		targetPos.Y -= speed
	case MovDown:
		targetPos.Y += speed
	case MovLeft:
		targetPos.X -= speed
	case MovRight:
		targetPos.X += speed
	}

	// Simulate entity movement with collision rules
	resultPos := s.moveCollider(sourcePos, targetPos, altitude)
	targetTile, _ = s.GetTile(resultPos)

	// Move the entity
	s.entitiesLock.Lock()
	entityData := sourceTile.PopEntity()
	targetTile.SetEntity(entityData)
	s.entities[entityID] = resultPos
	s.entitiesLock.Unlock()

	return nil
}

func (s *State) moveCollider(sourcePos Coordinates, targetPos Coordinates, altitude int) (result Coordinates) {
	// Keep track of our movements
	result = sourcePos
	checkPos := sourcePos
	// Loop counter is simply in case some bug causes an infinite loop
	// If anything moves a distance greater than twice the total board size
	// something is wrong
	for distanceMoved := 0; distanceMoved < s.size*2; distanceMoved++ {
		// move 1 towards out destination. If we're already at our destination
		// just return that
		switch {
		case targetPos.X > checkPos.X:
			checkPos.X += 1
		case targetPos.X < checkPos.X:
			checkPos.X -= 1
		case targetPos.Y > checkPos.Y:
			checkPos.Y += 1
		case targetPos.Y < checkPos.Y:
			checkPos.Y -= 1
		default: // Positions are the same
			return targetPos
		}

		// Get tile data for where we moved to
		checkTile, err := s.GetTile(checkPos)
		if err != nil {
			return result
		}

		// Make sure out target is free
		if checkTile.WillCollide(altitude) {
			return result
		}

		// Store that we successfully can move here
		result = checkPos
	}

	panic("movement calculation out of bounds!")
}

func outOfBounds(size int, pos Coordinates) bool {
	return pos.X > size-1 || pos.Y > size-1 || pos.X < 0 || pos.Y < 0
}

func forceBounds(dim int, max int) int {
	if dim < 0 {
		return 0
	}
	if dim > max {
		return max
	}

	return dim
}
